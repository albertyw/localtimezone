package localtimezone

import (
	"encoding/csv"
	"os"
	"strconv"
	"testing"
)

// city and lat/lon data is from Pareto Software LLC, SimpleMaps.com
// https://simplemaps.com/data/world-cities
type TimezoneTestCase struct {
	City         string
	Lat          float64
	Lon          float64
	ExpectedZone string
}

func generateTestCases() ([]TimezoneTestCase, error) {
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

	var data []TimezoneTestCase
	for _, line := range rawData {
		lat, err := strconv.ParseFloat(line[1], 64)
		if err != nil {
			return nil, err
		}
		lon, err := strconv.ParseFloat(line[2], 64)
		if err != nil {
			return nil, err
		}
		tc := TimezoneTestCase{
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
	z, err := NewLocalTimeZone()
	if err != nil {
		t.Errorf("cannot initialize timezone client: %v", err)
	}
	data, err := generateTestCases()
	if err != nil {
		t.Errorf("cannot get test data: %v", err)
	}
	for _, tc := range data {
		tc := tc // Remove race condition over test fields
		t.Run(tc.City, func(t *testing.T) {
			t.Parallel()
			point := Point{
				Lon: tc.Lon,
				Lat: tc.Lat,
			}
			tzid, err := z.GetZone(point)
			if err != nil {
				t.Errorf("unexpected err %v", err)
			}
			if len(tzid) < 1 {
				t.Error("cannot find a timezone")
			}
			if tc.ExpectedZone != tzid[0] {
				t.Errorf("expected zone %s; got %s", tc.ExpectedZone, tzid[0])
			}
		})
	}
}

func BenchmarkGetZone(b *testing.B) {
	client, err := NewLocalTimeZone()
	if err != nil {
		b.Errorf("cannot initialize timezone client: %v", err)
	}
	data, err := generateTestCases()
	if err != nil {
		b.Errorf("cannot initialize test cases: %v", err)
	}
	c := client.(*localTimeZone)
	c.mu.RLock()
	c.mu.RUnlock() //lint:ignore SA2001 Make sure client has loaded
	b.Run("city time zones", func(b *testing.B) {
	Loop:
		for n := 0; n < b.N; {
			for _, tc := range data {
				if n > b.N {
					break Loop
				}
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
		}
	})
}
