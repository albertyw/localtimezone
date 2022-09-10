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
	defer f.Close()

	reader := csv.NewReader(f)
	rawData, err := reader.ReadAll()
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
	data, err := generateTestCases()
	if err != nil {
		t.Errorf("cannot get test data: %v", err)
	}
	for _, tc := range data {
		t.Run(tc.City, func(t *testing.T) {
			point := Point{
				Lon: tc.Lon,
				Lat: tc.Lat,
			}
			tzid, err := GetZone(point)
			if err != nil {
				t.Errorf("unexpted err %v", err)
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
