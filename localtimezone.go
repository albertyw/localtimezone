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
	GetZone(p Point) (tzid []string, err error)
	LoadGeoJSON(io.Reader) error
}

type centers map[string][]orb.Point
type localTimeZone struct {
	orbData     *geojson.FeatureCollection
	boundCache  map[string]orb.Bound
	centerCache *centers
	mu          sync.RWMutex
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

	err = z.LoadGeoJSON(g)
	_ = g.Close()
	if err != nil {
		return err
	}
	return nil
}

// GetZone returns a slice of strings containing time zone id's for a given Point
func (z *localTimeZone) GetZone(point Point) (tzid []string, err error) {
	p := pointToOrb(point)
	if p[0] > 180 || p[0] < -180 || p[1] > 90 || p[1] < -90 {
		return nil, ErrOutOfRange
	}
	z.mu.RLock()
	defer z.mu.RUnlock()
	var wg sync.WaitGroup
	var tzidWriter sync.Mutex
	for _, v := range z.orbData.Features {
		wg.Add(1)
		go func(v *geojson.Feature) {
			defer wg.Done()
			id := v.Properties.MustString("tzid")
			if id == "" {
				return
			}
			if !z.boundCache[id].Contains(p) {
				return
			}
			polygon, ok := v.Geometry.(orb.Polygon)
			if ok {
				if planar.PolygonContains(polygon, p) {
					tzidWriter.Lock()
					tzid = append(tzid, id)
					tzidWriter.Unlock()
				}
				return
			}
			multiPolygon, ok := v.Geometry.(orb.MultiPolygon)
			if ok {
				if planar.MultiPolygonContains(multiPolygon, p) {
					tzidWriter.Lock()
					tzid = append(tzid, id)
					tzidWriter.Unlock()
				}
			}
		}(v)
	}
	wg.Wait()
	if len(tzid) > 0 {
		sort.Strings(tzid)
		return tzid, nil
	}
	return z.getClosestZone(p)
}

func (z *localTimeZone) getClosestZone(point orb.Point) (tzid []string, err error) {
	mindist := math.Inf(1)
	var winner string
	for id, v := range *z.centerCache {
		for _, p := range v {
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
	return append(tzid, winner), nil
}

func getNauticalZone(point orb.Point) (tzid []string, err error) {
	z := point[0] / 7.5
	z = (math.Abs(z) + 1) / 2
	z = math.Floor(z)
	if z == 0 {
		return append(tzid, "Etc/GMT"), nil
	}
	if point[0] < 0 {
		return append(tzid, fmt.Sprintf("Etc/GMT+%.f", z)), nil
	}
	return append(tzid, fmt.Sprintf("Etc/GMT-%.f", z)), nil
}

// buildCache builds centers for polygons
func (z *localTimeZone) buildCache() {
	centerCache := make(centers)
	z.boundCache = make(map[string]orb.Bound)
	var wg sync.WaitGroup
	var m sync.Mutex
	for _, v := range z.orbData.Features {
		wg.Add(1)
		go func(v *geojson.Feature) {
			defer wg.Done()
			tzid := v.Properties.MustString("tzid")
			if tzid == "" {
				return
			}
			var multiPolygon orb.MultiPolygon
			polygon, ok := v.Geometry.(orb.Polygon)
			if ok {
				multiPolygon = []orb.Polygon{polygon}
			} else {
				multiPolygon, _ = v.Geometry.(orb.MultiPolygon)
			}
			var tzCenters []orb.Point
			for _, polygon := range multiPolygon {
				for _, ring := range polygon {
					point, _ := planar.CentroidArea(ring)
					tzCenters = append(tzCenters, point)
				}
			}
			bound := v.Geometry.Bound()
			m.Lock()
			centerCache[tzid] = tzCenters
			z.boundCache[tzid] = bound
			m.Unlock()
		}(v)
	}
	wg.Wait()
	z.centerCache = &centerCache
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
		z.orbData = &geojson.FeatureCollection{}
		centerCache := make(centers)
		z.centerCache = &centerCache
		z.boundCache = make(map[string]orb.Bound)
		z.mu.Unlock()
		return err
	}
	z.orbData = orbData

	go func() {
		defer z.mu.Unlock()
		z.buildCache()
	}()
	return nil
}
