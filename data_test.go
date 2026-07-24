package localtimezone

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/uber/h3-go/v4"
)

// city and lat/lon data is from Pareto Software LLC, SimpleMaps.com
// https://simplemaps.com/data/world-cities
type timezoneTestCase struct {
	City         string
	Lat          float64
	Lon          float64
	ExpectedZone string
}

func generateTestCases() ([]timezoneTestCase, error) {
	f, err := os.Open("test/testdata.csv")
	if err != nil {
		return nil, err
	}

	reader := csv.NewReader(f)
	rawData, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}
	err = f.Close()
	if err != nil {
		return nil, err
	}

	var data []timezoneTestCase
	for _, line := range rawData {
		lat, err := strconv.ParseFloat(line[1], 64)
		if err != nil {
			return nil, err
		}
		lon, err := strconv.ParseFloat(line[2], 64)
		if err != nil {
			return nil, err
		}
		tc := timezoneTestCase{
			City:         line[0],
			Lat:          lat,
			Lon:          lon,
			ExpectedZone: line[3],
		}
		data = append(data, tc)
	}
	return data, nil
}

func TestData(t *testing.T) {
	t.Parallel()
	z := NewLocalTimeZone()
	data, err := generateTestCases()
	if err != nil {
		t.Errorf("cannot get test data: %v", err)
	}
	for _, tc := range data {
		t.Run(tc.City, func(t *testing.T) {
			t.Parallel()
			point := Point{
				Lon: tc.Lon,
				Lat: tc.Lat,
			}
			tzids, err := z.GetZone(point)
			if err != nil {
				t.Errorf("unexpected err %v", err)
			}
			if len(tzids) < 1 {
				t.Error("cannot find a timezone")
			}
			if tc.ExpectedZone != tzids[0] {
				t.Errorf("expected zone %s; got %s", tc.ExpectedZone, tzids[0])
			}
			tzid, err := z.GetOneZone(point)
			if err != nil {
				t.Errorf("unexpected err %v", err)
			}
			if tc.ExpectedZone != tzid {
				t.Errorf("expected zone %s; got %s", tc.ExpectedZone, tzid)
			}
		})
	}
}

func TestTzNamesPresent(t *testing.T) {
	client := NewLocalTimeZone()
	z, ok := client.(*localTimeZone)
	if !ok {
		t.Error("error when initializing client")
	}
	cache := z.data.Load()
	if len(cache.tzNames) == 0 {
		t.Error("expected timezone names in cache")
	}
	for _, name := range cache.tzNames {
		if name == "" {
			t.Error("unexpected empty timezone name")
		}
	}
}

func TestZonesValidLoadLocation(t *testing.T) {
	t.Parallel()
	client := NewLocalTimeZone()
	z, ok := client.(*localTimeZone)
	if !ok {
		t.Error("error when initializing client")
	}

	// Every timezone name embedded in the data must be loadable by time.LoadLocation.
	for _, name := range z.data.Load().tzNames {
		if _, err := time.LoadLocation(name); err != nil {
			t.Errorf("timezone %q is not valid for time.LoadLocation: %v", name, err)
		}
	}

	// The nautical fallback zones are generated at lookup time rather than stored
	// in the data, so verify they are loadable across the full longitude range too.
	for lon := -180.0; lon <= 180.0; lon += 7.5 {
		zones, err := getNauticalZone(h3.NewLatLng(0, lon))
		if err != nil {
			t.Fatalf("cannot get nautical zone for lon %f: %v", lon, err)
		}
		for _, name := range zones {
			if _, err := time.LoadLocation(name); err != nil {
				t.Errorf("nautical timezone %q is not valid for time.LoadLocation: %v", name, err)
			}
		}
	}
}

func TestCellsPresent(t *testing.T) {
	client := NewLocalTimeZone()
	z, ok := client.(*localTimeZone)
	if !ok {
		t.Error("error when initializing client")
	}
	cache := z.data.Load()
	if len(cache.cells) == 0 {
		t.Error("expected cells in cache")
	}
	if len(cache.cells) != len(cache.tzIdx) {
		t.Errorf("cells and tzIdx length mismatch: %d vs %d", len(cache.cells), len(cache.tzIdx))
	}
	// Verify cells are sorted
	for i := 1; i < len(cache.cells); i++ {
		if cache.cells[i] < cache.cells[i-1] {
			t.Errorf("cells not sorted at index %d", i)
			break
		}
	}
}

func BenchmarkGetZone(b *testing.B) {
	client := NewLocalTimeZone()
	data, err := generateTestCases()
	if err != nil {
		b.Errorf("cannot initialize test cases: %v", err)
	}

	// Ensure client has finished loading data
	_, err = client.GetZone(Point{0, 0})
	if err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}

	b.Run("GetZone on large cities", func(b *testing.B) {
		n := 0
		for b.Loop() {
			tc := data[n%len(data)]
			point := Point{
				Lon: tc.Lon,
				Lat: tc.Lat,
			}
			_, err = client.GetZone(point)
			if err != nil {
				b.Errorf("point %v did not return a zone", point)
			}
			n++
		}
	})
	b.Run("GetOneZone on large cities", func(b *testing.B) {
		n := 0
		for b.Loop() {
			tc := data[n%len(data)]
			point := Point{
				Lon: tc.Lon,
				Lat: tc.Lat,
			}
			_, err = client.GetOneZone(point)
			if err != nil {
				b.Errorf("point %v did not return a zone", point)
			}
			n++
		}
	})
}
