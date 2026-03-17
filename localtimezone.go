// Package localtimezone provides timezone lookup for a given location
//
// # Features
//
// * The timezone shapefile is embedded in the build binary using //go:embed
//
// * Supports overlapping zones
//
// * You can load your own geojson shapefile if you want
//
// * Sub millisecond lookup even on old hardware
//
// # Problems
//
// * The shapefile is simplified using a lossy method so it may be innacurate along the borders
//
// * This is purely in-memory. Uses ~50MB of ram
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
	"sync"
	"sync/atomic"

	"github.com/klauspost/compress/gzip"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/encoding/wkb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

// TZShapeFile is the data containing geographic shapes for timezone borders.
// This data is WKB binary format compressed with gzip.
//
//go:embed data.wkb.gz
var TZShapeFile []byte

// MockTZShapeFile is similar to TZShapeFile but maps the entire world to the timezone America/Los_Angeles.
// This data is WKB binary format compressed with gzip.
// It is meant for testing.
//
//go:embed data_mock.wkb.gz
var MockTZShapeFile []byte

// MockTimeZone is the timezone that is always returned from the NewMockLocalTimeZone client
const MockTimeZone = "America/Los_Angeles"

// ErrOutOfRange is returned when latitude exceeds 90 degrees or longitude exceeds 180 degrees
var ErrOutOfRange = errors.New("point's coordinates out of range")

// ErrNoTimeZone is returned when no matching timezone is found
// This error should never be returned because the client will attempt to return the nearest zone
var ErrNoTimeZone = errors.New("no timezone found")

// Point describes a location by Latitude and Longitude
type Point struct {
	Lon float64
	Lat float64
}

// pointFromOrb converts an orb Point into an internal Point
func pointFromOrb(p orb.Point) Point {
	return Point{Lon: p[0], Lat: p[1]}
}

// pointToOrb converts an internal Point to an orb Point
func pointToOrb(p Point) orb.Point {
	return orb.Point{p.Lon, p.Lat}
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

type tzData struct {
	id           string
	polygon      *orb.Polygon
	multiPolygon *orb.MultiPolygon
	bound        *orb.Bound
	centers      []orb.Point
}

const (
	gridLonCells = 360
	gridLatCells = 180
)

type spatialGrid struct {
	// cells stores tzData indices for each grid cell.
	// Indexed as [lonCell * gridLatCells + latCell].
	cells [][]int
}

func newSpatialGrid() *spatialGrid {
	return &spatialGrid{
		cells: make([][]int, gridLonCells*gridLatCells),
	}
}

// cellIndex returns the grid cell index for a given lon/lat point.
func cellIndex(lon, lat float64) int {
	lonCell := int(lon + 180)
	if lonCell >= gridLonCells {
		lonCell = gridLonCells - 1
	}
	latCell := int(lat + 90)
	if latCell >= gridLatCells {
		latCell = gridLatCells - 1
	}
	return lonCell*gridLatCells + latCell
}

// gridCellThreshold is the maximum number of grid cells a bound can span
// before the zone is added to the wideIndices list instead.
const gridCellThreshold = gridLonCells * gridLatCells / 4

// insert adds a tzData index to all grid cells that overlap the given bound.
// Returns true if the index was inserted into the grid, false if the bound
// was too wide (caller should add to wideIndices instead).
func (g *spatialGrid) insert(idx int, bound orb.Bound) bool {
	minLon := int(math.Floor(bound.Min[0] + 180))
	maxLon := int(math.Floor(bound.Max[0] + 180))
	minLat := int(math.Floor(bound.Min[1] + 90))
	maxLat := int(math.Floor(bound.Max[1] + 90))
	if minLon < 0 {
		minLon = 0
	}
	if maxLon >= gridLonCells {
		maxLon = gridLonCells - 1
	}
	if minLat < 0 {
		minLat = 0
	}
	if maxLat >= gridLatCells {
		maxLat = gridLatCells - 1
	}
	cellCount := (maxLon - minLon + 1) * (maxLat - minLat + 1)
	if cellCount > gridCellThreshold {
		return false
	}
	for lon := minLon; lon <= maxLon; lon++ {
		for lat := minLat; lat <= maxLat; lat++ {
			ci := lon*gridLatCells + lat
			g.cells[ci] = append(g.cells[ci], idx)
		}
	}
	return true
}

// candidates returns the tzData indices for the grid cell containing the point.
func (g *spatialGrid) candidates(lon, lat float64) []int {
	return g.cells[cellIndex(lon, lat)]
}

type immutableCache struct {
	tzData      []tzData
	grid        *spatialGrid
	wideIndices []int
}

type localTimeZone struct {
	data atomic.Pointer[immutableCache]
}

var _ LocalTimeZone = &localTimeZone{}

// NewLocalTimeZone creates a new LocalTimeZone with real timezone data
// The client is threadsafe
func NewLocalTimeZone() (LocalTimeZone, error) {
	z := localTimeZone{}
	err := z.load(TZShapeFile)
	return &z, err
}

// NewMockLocalTimeZone creates a new LocalTimeZone that always returns
// America/Los_Angeles as the timezone
// The client is threadsafe
func NewMockLocalTimeZone() LocalTimeZone {
	z := localTimeZone{}
	err := z.load(MockTZShapeFile)
	if err != nil {
		// The MockTZShapeFile is embedded and designed to never panic
		panic(err)
	}
	return &z
}

func (z *localTimeZone) load(shapeFile []byte) error {
	g, err := gzip.NewReader(bytes.NewBuffer(shapeFile))
	if err != nil {
		return err
	}
	defer g.Close()

	return z.loadWKB(g)
}

func (z *localTimeZone) loadWKB(r io.Reader) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	buf := bytes.NewReader(data)

	var featureCount uint32
	if err := binary.Read(buf, binary.LittleEndian, &featureCount); err != nil {
		return err
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	tzDatas := make([]tzData, 0, featureCount)

	for i := uint32(0); i < featureCount; i++ {
		var tzidLen uint16
		if err := binary.Read(buf, binary.LittleEndian, &tzidLen); err != nil {
			return err
		}
		tzidBytes := make([]byte, tzidLen)
		if _, err := io.ReadFull(buf, tzidBytes); err != nil {
			return err
		}
		tzid := string(tzidBytes)

		var wkbLen uint32
		if err := binary.Read(buf, binary.LittleEndian, &wkbLen); err != nil {
			return err
		}
		wkbBytes := make([]byte, wkbLen)
		if _, err := io.ReadFull(buf, wkbBytes); err != nil {
			return err
		}

		geometry, err := wkb.Unmarshal(wkbBytes)
		if err != nil {
			return err
		}

		wg.Add(1)
		go func(tzid string, geometry orb.Geometry) {
			defer wg.Done()
			d := tzData{id: tzid}
			var multiPolygon orb.MultiPolygon
			polygon, ok := geometry.(orb.Polygon)
			if ok {
				d.polygon = &polygon
				multiPolygon = []orb.Polygon{polygon}
			} else {
				mp, _ := geometry.(orb.MultiPolygon)
				d.multiPolygon = &mp
				multiPolygon = mp
			}
			var tzCenters []orb.Point
			for _, polygon := range multiPolygon {
				for _, ring := range polygon {
					point, _ := planar.CentroidArea(ring)
					tzCenters = append(tzCenters, point)
				}
			}
			bound := geometry.Bound()
			d.bound = &bound
			d.centers = tzCenters
			mu.Lock()
			tzDatas = append(tzDatas, d)
			mu.Unlock()
		}(tzid, geometry)
	}
	wg.Wait()

	sort.Slice(tzDatas, func(i, j int) bool {
		return tzDatas[i].id < tzDatas[j].id
	})
	cache := immutableCache{tzData: tzDatas}
	z.data.Store(&cache)
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
	p := pointToOrb(point)
	if p[0] > 180 || p[0] < -180 || p[1] > 90 || p[1] < -90 {
		return nil, ErrOutOfRange
	}
	cache := z.data.Load()
	candidates := cache.grid.candidates(p[0], p[1])
	for i := 0; i < len(candidates)+len(cache.wideIndices); i++ {
		var idx int
		if i < len(candidates) {
			idx = candidates[i]
		} else {
			idx = cache.wideIndices[i-len(candidates)]
		}
		d := &cache.tzData[idx]
		if !d.bound.Contains(p) {
			continue
		}
		if d.polygon != nil {
			if planar.PolygonContains(*d.polygon, p) {
				tzids = append(tzids, d.id)
				if single {
					return
				}
			}
			continue
		}
		if d.multiPolygon != nil {
			if planar.MultiPolygonContains(*d.multiPolygon, p) {
				tzids = append(tzids, d.id)
				if single {
					return
				}
			}
		}
	}
	if len(tzids) > 0 {
		return tzids, nil
	}
	return z.getClosestZone(p, cache)
}

func (z *localTimeZone) getClosestZone(point orb.Point, cache *immutableCache) (tzids []string, err error) {
	mindist := math.Inf(1)
	var winner string
	for _, d := range cache.tzData {
		for _, p := range d.centers {
			tmp := planar.Distance(p, point)
			if tmp < mindist {
				mindist = tmp
				winner = d.id
			}
		}
	}
	// Limit search radius
	if mindist > 2.0 {
		return getNauticalZone(point)
	}
	return append(tzids, winner), nil
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

// buildCache builds centers for polygons
func (z *localTimeZone) buildCache(features []*geojson.Feature) {
	var wg sync.WaitGroup
	var m sync.Mutex
	tzDatas := make([]tzData, 0, len(features))
	for _, f := range features {
		wg.Add(1)
		go func(f *geojson.Feature) {
			defer wg.Done()
			id := f.Properties.MustString("tzid")
			var multiPolygon orb.MultiPolygon
			d := tzData{id: id}
			polygon, ok := f.Geometry.(orb.Polygon)
			if ok {
				d.polygon = &polygon
				multiPolygon = []orb.Polygon{polygon}
			} else {
				multiPolygon, _ = f.Geometry.(orb.MultiPolygon)
				d.multiPolygon = &multiPolygon
			}
			var tzCenters []orb.Point
			for _, polygon := range multiPolygon {
				for _, ring := range polygon {
					point, _ := planar.CentroidArea(ring)
					tzCenters = append(tzCenters, point)
				}
			}
			bound := f.Geometry.Bound()
			d.bound = &bound
			d.centers = tzCenters
			m.Lock()
			tzDatas = append(tzDatas, d)
			m.Unlock()
		}(f)
	}
	wg.Wait()

	sort.Slice(tzDatas, func(i, j int) bool {
		return tzDatas[i].id < tzDatas[j].id
	})
	grid := newSpatialGrid()
	var wideIndices []int
	for i, d := range tzDatas {
		if !grid.insert(i, *d.bound) {
			wideIndices = append(wideIndices, i)
		}
	}
	cache := immutableCache{tzData: tzDatas, grid: grid, wideIndices: wideIndices}
	z.data.Store(&cache)
}

// LoadGeoJSON loads a custom GeoJSON shapefile from a Reader
func (z *localTimeZone) LoadGeoJSON(r io.Reader) error {
	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}
	orbData, err := geojson.UnmarshalFeatureCollection(buf.Bytes())
	if err != nil {
		cache := immutableCache{tzData: []tzData{}, grid: newSpatialGrid(), wideIndices: nil}
		z.data.Store(&cache)
		return err
	}
	z.buildCache(orbData.Features)
	return nil
}
