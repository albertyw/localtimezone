// Package localtimezone provides timezone lookup for a given location
//
// # Features
//
// * The timezone data is embedded in the build binary using //go:embed
//
// * Supports overlapping zones
//
// * You can load your own geojson data if you want
//
// * Sub millisecond lookup even on old hardware
//
// # Problems
//
// * H3 hexagonal discretization may be inaccurate along timezone borders
//
// * This is purely in-memory
package localtimezone

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"sync/atomic"

	"github.com/klauspost/compress/s2"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/uber/h3-go/v4"
)

// TZData is the data containing H3 cell-to-timezone mappings.
// This data is H3 binary format compressed with S2.
//
//go:embed data.h3.s2
var TZData []byte

// MockTZData is similar to TZData but maps the entire world to the timezone America/Los_Angeles.
// This data is H3 binary format compressed with S2.
// It is meant for testing.
//
//go:embed data_mock.h3.s2
var MockTZData []byte

// MockTimeZone is the timezone that is always returned from the NewMockLocalTimeZone client
const MockTimeZone = "America/Los_Angeles"

// ErrOutOfRange is returned when latitude exceeds 90 degrees or longitude exceeds 180 degrees
var ErrOutOfRange = errors.New("point's coordinates out of range")

// ErrNoTimeZone is returned when no matching timezone is found
// This error should never be returned because the client will attempt to return the nearest zone
var ErrNoTimeZone = errors.New("no timezone found")

const maxFallbackRings = 3

// Point describes a location by Latitude and Longitude
type Point struct {
	Lon float64
	Lat float64
}

func init() {
	// Set a faster json unmarshaller
	geojson.CustomJSONUnmarshaler = unmarshaler{}
}

// LocalTimeZone is a client for looking up time zones by Points
type LocalTimeZone interface {
	GetZone(p Point) (tzids []string, err error)
	GetOneZone(p Point) (tzid string, err error)
	LoadGeoJSON(io.Reader) error
}

type immutableCache struct {
	tzNames    []string // string table from binary
	cells      []int64  // sorted H3 cell IDs
	tzIdx      []uint16 // parallel array: tzNames index for each cell
	resolution int      // H3 resolution used for generation
}

type localTimeZone struct {
	data atomic.Pointer[immutableCache]
}

var _ LocalTimeZone = &localTimeZone{}

// NewLocalTimeZone creates a new LocalTimeZone with real timezone data
// The client is threadsafe
func NewLocalTimeZone() (LocalTimeZone, error) {
	z := localTimeZone{}
	err := z.load(TZData)
	return &z, err
}

// NewMockLocalTimeZone creates a new LocalTimeZone that always returns
// America/Los_Angeles as the timezone
// The client is threadsafe
func NewMockLocalTimeZone() LocalTimeZone {
	z := localTimeZone{}
	err := z.load(MockTZData)
	if err != nil {
		// The MockTZData is embedded and designed to never panic
		panic(err)
	}
	return &z
}

func (z *localTimeZone) load(data []byte) error {
	return z.loadH3(s2.NewReader(bytes.NewReader(data)))
}

func (z *localTimeZone) loadH3(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	// Minimum size: 4 (magic) + 1 (version) + 1 (resolution) + 2 (string count) = 8
	if len(data) < 8 {
		return fmt.Errorf("data too short: %d bytes", len(data))
	}

	// Read header directly from byte slice
	if string(data[0:4]) != "H3TZ" {
		return fmt.Errorf("invalid magic: %q", data[0:4])
	}
	version := data[4]
	if version != 1 {
		return fmt.Errorf("unsupported version: %d", version)
	}
	resolution := data[5]
	stringCount := binary.LittleEndian.Uint16(data[6:8])
	off := 8

	// Read string table
	tzNames := make([]string, stringCount)
	for i := uint16(0); i < stringCount; i++ {
		if off+2 > len(data) {
			return fmt.Errorf("unexpected end of data reading string table")
		}
		strLen := int(binary.LittleEndian.Uint16(data[off : off+2]))
		off += 2
		if off+strLen > len(data) {
			return fmt.Errorf("unexpected end of data reading string")
		}
		tzNames[i] = string(data[off : off+strLen])
		off += strLen
	}

	// Read cell count
	if off+4 > len(data) {
		return fmt.Errorf("unexpected end of data reading cell count")
	}
	cellCount := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4

	// Bulk read: each entry is 10 bytes (8 for int64 cell + 2 for uint16 tz index)
	const entrySize = 10
	cellDataLen := int(cellCount) * entrySize
	if off+cellDataLen > len(data) {
		return fmt.Errorf("unexpected end of data reading cells")
	}
	cellData := data[off : off+cellDataLen]

	cells := make([]int64, cellCount)
	tzIdx := make([]uint16, cellCount)
	for i := 0; i < int(cellCount); i++ {
		base := i * entrySize
		cells[i] = int64(binary.LittleEndian.Uint64(cellData[base : base+8]))
		tzIdx[i] = binary.LittleEndian.Uint16(cellData[base+8 : base+10])
	}

	cache := &immutableCache{
		tzNames:    tzNames,
		cells:      cells,
		tzIdx:      tzIdx,
		resolution: int(resolution),
	}
	z.data.Store(cache)
	return nil
}

// GetZone returns a slice of strings containing time zone id's for a given Point
func (z *localTimeZone) GetZone(point Point) (tzids []string, err error) {
	return z.getZone(point, false)
}

// GetOneZone returns a single zone id for a given Point
func (z *localTimeZone) GetOneZone(point Point) (tzid string, err error) {
	tzids, err := z.getZone(point, true)
	if err != nil {
		return "", err
	}
	if len(tzids) == 0 {
		return "", ErrNoTimeZone
	}
	return tzids[0], err
}

func (z *localTimeZone) getZone(point Point, single bool) (tzids []string, err error) {
	if point.Lon > 180 || point.Lon < -180 || point.Lat > 90 || point.Lat < -90 {
		return nil, ErrOutOfRange
	}

	cache := z.data.Load()
	latLng := h3.NewLatLng(point.Lat, point.Lon)
	cell, err := h3.LatLngToCell(latLng, cache.resolution)
	if err != nil {
		return nil, err
	}

	// Check all resolutions from finest to coarsest (for compacted cells)
	for res := cache.resolution; res >= 0; res-- {
		var lookup h3.Cell
		if res == cache.resolution {
			lookup = cell
		} else {
			var err error
			lookup, err = cell.Parent(res)
			if err != nil {
				// Skip this resolution; other resolutions may still match
				continue
			}
		}
		matches := z.findCell(lookup, cache)
		tzids = append(tzids, matches...)
		if single && len(tzids) > 0 {
			return tzids[:1], nil
		}
	}
	if len(tzids) > 0 {
		return tzids, nil
	}

	return z.getClosestZone(cell, cache)
}

// findCell returns all timezone names matching a cell via binary search.
// Since the cells array may contain duplicate cell values (for overlapping zones),
// it scans forward from the first matching index returned by sort.Search.
func (z *localTimeZone) findCell(cell h3.Cell, cache *immutableCache) []string {
	cellVal := int64(cell)
	idx := sort.Search(len(cache.cells), func(i int) bool {
		return cache.cells[i] >= cellVal
	})
	if idx >= len(cache.cells) || cache.cells[idx] != cellVal {
		return nil
	}

	var results []string
	// Scan forward from idx to collect all entries with same cell
	for i := idx; i < len(cache.cells) && cache.cells[i] == cellVal; i++ {
		results = append(results, cache.tzNames[cache.tzIdx[i]])
	}
	return results
}

func (z *localTimeZone) getClosestZone(cell h3.Cell, cache *immutableCache) ([]string, error) {
	// Expanding ring search
	for k := 1; k <= maxFallbackRings; k++ {
		ring, err := cell.GridDisk(k)
		if err != nil {
			// Skip this ring distance; try the next larger ring
			continue
		}
		for _, neighbor := range ring {
			// Check all resolutions for each neighbor
			for res := cache.resolution; res >= 0; res-- {
				var lookup h3.Cell
				if res == cache.resolution {
					lookup = neighbor
				} else {
					var err error
					lookup, err = neighbor.Parent(res)
					if err != nil {
						// Skip this resolution; other resolutions may still match
						continue
					}
				}
				matches := z.findCell(lookup, cache)
				if len(matches) > 0 {
					return matches[:1], nil
				}
			}
		}
	}
	// Final fallback: nautical zone
	latLng, _ := cell.LatLng()
	return getNauticalZone(orb.Point{latLng.Lng, latLng.Lat})
}

func getNauticalZone(point orb.Point) (tzids []string, err error) {
	z := point[0] / 7.5
	z = (math.Abs(z) + 1) / 2
	z = math.Floor(z)
	if z == 0 {
		return append(tzids, "Etc/GMT"), nil
	}
	if point[0] < 0 {
		return append(tzids, fmt.Sprintf("Etc/GMT+%.f", z)), nil
	}
	return append(tzids, fmt.Sprintf("Etc/GMT-%.f", z)), nil
}

// LoadGeoJSON loads custom GeoJSON timezone data from a Reader
func (z *localTimeZone) LoadGeoJSON(r io.Reader) error {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}
	orbData, err := geojson.UnmarshalFeatureCollection(buf.Bytes())
	if err != nil {
		cache := &immutableCache{
			tzNames: []string{},
			cells:   []int64{},
			tzIdx:   []uint16{},
		}
		z.data.Store(cache)
		return err
	}
	z.buildCacheFromGeoJSON(orbData.Features)
	return nil
}

// buildCacheFromGeoJSON converts GeoJSON features to H3 cells at runtime
func (z *localTimeZone) buildCacheFromGeoJSON(features []*geojson.Feature) {
	const defaultResolution = 7

	type cellEntry struct {
		cell  int64
		tzIdx uint16
	}

	// Build string table
	tzNameMap := make(map[string]uint16)
	var tzNames []string
	for _, f := range features {
		id := f.Properties.MustString("tzid")
		if _, exists := tzNameMap[id]; !exists {
			tzNameMap[id] = uint16(len(tzNames))
			tzNames = append(tzNames, id)
		}
	}

	// Convert each feature's geometry to H3 cells
	var entries []cellEntry
	for _, f := range features {
		id := f.Properties.MustString("tzid")
		idx := tzNameMap[id]

		var polygons []orb.Polygon
		switch g := f.Geometry.(type) {
		case orb.Polygon:
			polygons = []orb.Polygon{g}
		case orb.MultiPolygon:
			polygons = []orb.Polygon(g)
		default:
			continue
		}

		for _, polygon := range polygons {
			geoPolygon := orbPolygonToH3(polygon)
			if len(geoPolygon.GeoLoop) == 0 {
				continue
			}
			cells, err := h3.PolygonToCells(geoPolygon, defaultResolution)
			if err != nil {
				continue
			}
			for _, c := range cells {
				entries = append(entries, cellEntry{cell: int64(c), tzIdx: idx})
			}
		}
	}

	// Sort by cell value
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].cell == entries[j].cell {
			return entries[i].tzIdx < entries[j].tzIdx
		}
		return entries[i].cell < entries[j].cell
	})

	cells := make([]int64, len(entries))
	tzIdx := make([]uint16, len(entries))
	for i, e := range entries {
		cells[i] = e.cell
		tzIdx[i] = e.tzIdx
	}

	cache := &immutableCache{
		tzNames:    tzNames,
		cells:      cells,
		tzIdx:      tzIdx,
		resolution: defaultResolution,
	}
	z.data.Store(cache)
}

// orbPolygonToH3 converts an orb.Polygon to an h3.GeoPolygon
func orbPolygonToH3(polygon orb.Polygon) h3.GeoPolygon {
	if len(polygon) == 0 {
		return h3.GeoPolygon{}
	}
	outer := make(h3.GeoLoop, len(polygon[0]))
	for i, pt := range polygon[0] {
		outer[i] = h3.NewLatLng(pt[1], pt[0]) // orb: [lon, lat], h3: (lat, lng)
	}
	var holes []h3.GeoLoop
	for _, ring := range polygon[1:] {
		hole := make(h3.GeoLoop, len(ring))
		for i, pt := range ring {
			hole[i] = h3.NewLatLng(pt[1], pt[0])
		}
		holes = append(holes, hole)
	}
	return h3.GeoPolygon{GeoLoop: outer, Holes: holes}
}
