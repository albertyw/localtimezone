// Package localtimezone provides timezone lookup for a given location
//
// # Features
//
// * The timezone data is embedded in the build binary using //go:embed
//
// * Supports overlapping zones
//
// * Microsecond-level lookup even on old hardware
//
// # Problems
//
// * H3 hexagonal discretization may be inaccurate along timezone borders
//
// * This is purely in-memory
package localtimezone

import (
	_ "embed"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"sync/atomic"

	"github.com/klauspost/compress/s2"
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

// LocalTimeZone is a client for looking up time zones by Points
type LocalTimeZone interface {
	GetZone(p Point) (tzids []string, err error)
	GetOneZone(p Point) (tzid string, err error)
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

// NewLocalTimeZone creates a new LocalTimeZone with real timezone data.
// The client is threadsafe.
// Init is deterministic: TZData is a fixed embedded binary, so every call
// produces an equivalent client.
func NewLocalTimeZone() LocalTimeZone {
	return newLocalTimeZone(TZData)
}

// NewMockLocalTimeZone creates a new LocalTimeZone that always returns
// America/Los_Angeles as the timezone
// The client is threadsafe
func NewMockLocalTimeZone() LocalTimeZone {
	return newLocalTimeZone(MockTZData)
}

func newLocalTimeZone(data []byte) LocalTimeZone {
	z := localTimeZone{}
	if err := z.load(data); err != nil {
		// Unreachable: the data is embedded at compile time and always valid.
		panic(err)
	}
	return &z
}

func (z *localTimeZone) load(dataCompressed []byte) error {
	data, err := s2.Decode(nil, dataCompressed)
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
	for i := range stringCount {
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
	for i := range int(cellCount) {
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

	for res := cache.resolution; res >= 0; res-- {
		lookup, err := lookupCell(cell, res, cache.resolution)
		if err != nil {
			// Skip this resolution; other resolutions may still match
			continue
		}
		for _, m := range z.findCell(lookup, cache) {
			if single {
				return []string{m}, nil
			}
			if !slices.Contains(tzids, m) {
				tzids = append(tzids, m)
			}
		}
	}
	if len(tzids) > 0 {
		return tzids, nil
	}

	return z.getClosestZone(cell, cache)
}

// lookupCell returns the cell to search for at resolution res. Cells are
// stored compacted, so a cell may only be present as one of its ancestors and
// every resolution from the data's own down to 0 has to be checked.
func lookupCell(cell h3.Cell, res, resolution int) (h3.Cell, error) {
	if res == resolution {
		return cell, nil
	}
	return parentCell(cell, res)
}

// An H3 index stores its resolution in bits 52-55 and one 3-bit digit per
// resolution 1 through 15 in the low 45 bits. Digits finer than the index's
// own resolution are all ones.
const (
	h3ResolutionOffset = 52
	h3ResolutionMask   = uint64(0xf) << h3ResolutionOffset
	h3MaxResolution    = 15
	h3DigitBits        = 3
)

// parentCell returns the ancestor of cell at resolution res. It is a pure Go
// equivalent of h3's cellToParent: rewrite the resolution field and blank
// every digit finer than res. h3-go is a cgo binding, and getZone walks up to
// eight resolutions per lookup, so avoiding the cgo call here is worth the
// duplicated knowledge of the index layout.
func parentCell(cell h3.Cell, res int) (h3.Cell, error) {
	index := uint64(cell)
	resolution := int((index & h3ResolutionMask) >> h3ResolutionOffset)
	if res < 0 || res > resolution {
		return 0, fmt.Errorf("resolution %d is not an ancestor of resolution %d", res, resolution)
	}
	index = index&^h3ResolutionMask | uint64(res)<<h3ResolutionOffset
	// Unused digits are all ones, so every digit finer than res blanks out as
	// a single run of set bits.
	index |= uint64(1)<<((h3MaxResolution-res)*h3DigitBits) - 1
	return h3.Cell(index), nil
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
			for res := cache.resolution; res >= 0; res-- {
				lookup, err := lookupCell(neighbor, res, cache.resolution)
				if err != nil {
					// Skip this resolution; other resolutions may still match
					continue
				}
				if matches := z.findCell(lookup, cache); len(matches) > 0 {
					return matches[:1], nil
				}
			}
		}
	}
	// Final fallback: nautical zone
	latLng, _ := cell.LatLng()
	return getNauticalZone(latLng)
}

// getNauticalZone returns the nautical timezone for a point, used when no
// timezone boundary covers it. Nautical zones are 15 degrees of longitude
// wide and centered on multiples of 15 degrees.
func getNauticalZone(point h3.LatLng) (tzids []string, err error) {
	offset := math.Floor((math.Abs(point.Lng/7.5) + 1) / 2)
	switch {
	case offset == 0:
		return []string{"Etc/GMT"}, nil
	case point.Lng < 0:
		return []string{fmt.Sprintf("Etc/GMT+%.f", offset)}, nil
	default:
		return []string{fmt.Sprintf("Etc/GMT-%.f", offset)}, nil
	}
}
