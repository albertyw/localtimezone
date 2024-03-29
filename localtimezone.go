// Package localtimezone provides timezone lookup for a given location
//
// # Features
//
// * The timezone shapefile is embedded in the build binary using go-bindata
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
	"compress/gzip"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"sync"

	json "github.com/json-iterator/go"
	"github.com/paulmach/orb"
	"github.com/paulmach/orb/geojson"
	"github.com/paulmach/orb/planar"
)

// TZShapeFile is the data containing geographic shapes for timezone borders.
// This data is a large json blob compressed with gzip.
//
//go:embed data.json.gz
var TZShapeFile []byte

// MockTZShapeFile is similar to TZShapeFile but maps the entire world to the timezone America/Los_Angeles.
// This data is a small json blob compressed with gzip.
// It is meant for testing.
//
//go:embed data_mock.json.gz
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
	geojson.CustomJSONUnmarshaler = json.ConfigFastest
}

// LocalTimeZone is a client for looking up time zones by Points
type LocalTimeZone interface {
	GetZone(p Point) (tzids []string, err error)
	GetOneZone(p Point) (tzid string, err error)
	LoadGeoJSON(io.Reader) error
}

type tzData struct {
	polygon      *orb.Polygon
	multiPolygon *orb.MultiPolygon
	bound        *orb.Bound
	centers      []orb.Point
}

type localTimeZone struct {
	tzids  []string
	tzData map[string]tzData
	mu     sync.RWMutex
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

	err = z.LoadGeoJSON(g)
	if err != nil {
		return err
	}
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
	z.mu.RLock()
	defer z.mu.RUnlock()
	for _, id := range z.tzids {
		d := z.tzData[id]
		if !d.bound.Contains(p) {
			continue
		}
		if d.polygon != nil {
			if planar.PolygonContains(*d.polygon, p) {
				tzids = append(tzids, id)
				if single {
					return
				}
			}
			continue
		}
		if d.multiPolygon != nil {
			if planar.MultiPolygonContains(*d.multiPolygon, p) {
				tzids = append(tzids, id)
				if single {
					return
				}
			}
		}
	}
	if len(tzids) > 0 {
		return tzids, nil
	}
	return z.getClosestZone(p)
}

func (z *localTimeZone) getClosestZone(point orb.Point) (tzids []string, err error) {
	mindist := math.Inf(1)
	var winner string
	for id, d := range z.tzData {
		for _, p := range d.centers {
			tmp := planar.Distance(p, point)
			if tmp < mindist {
				mindist = tmp
				winner = id
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
	m.Lock()
	for _, f := range features {
		wg.Add(1)
		go func(f *geojson.Feature) {
			defer wg.Done()
			id := f.Properties.MustString("tzid")
			var multiPolygon orb.MultiPolygon
			d := tzData{}
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
			z.tzData[id] = d
			m.Unlock()
		}(f)
	}
	m.Unlock()
	wg.Wait()

	z.tzids = make([]string, len(z.tzData))
	i := 0
	for tzid := range z.tzData {
		z.tzids[i] = tzid
		i++
	}
	sort.Strings(z.tzids)
}

// LoadGeoJSON loads a custom GeoJSON shapefile from a Reader
func (z *localTimeZone) LoadGeoJSON(r io.Reader) error {
	z.mu.Lock()

	var buf bytes.Buffer
	_, err := buf.ReadFrom(r)
	if err != nil {
		return err
	}
	orbData, err := geojson.UnmarshalFeatureCollection(buf.Bytes())
	if err != nil {
		z.tzData = make(map[string]tzData)
		z.tzids = []string{}
		z.mu.Unlock()
		return err
	}
	z.tzData = make(map[string]tzData, TZCount) // Possibly the incorrect length in case of Mock or custom data
	z.tzids = []string{}                        // Cannot set a length or else array will be full of empty strings
	go func(features []*geojson.Feature) {
		defer z.mu.Unlock()
		z.buildCache(features)
	}(orbData.Features)
	return nil
}
