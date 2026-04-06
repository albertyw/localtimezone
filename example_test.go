package localtimezone_test

import (
	"fmt"

	localtimezone "github.com/albertyw/localtimezone/v4"
)

func ExampleLocalTimeZone_GetZone() {
	z := localtimezone.NewLocalTimeZone()
	// Loading zones for the Alaska panhandle (overlaps America/Sitka and America/Vancouver)
	p := localtimezone.Point{
		Lon: -132.783555, Lat: 54.554439,
	}
	zones, err := z.GetZone(p)
	if err != nil {
		panic(err)
	}
	for _, zone := range zones {
		fmt.Println(zone)
	}
	// Output:
	// America/Sitka
	// America/Vancouver
}

func ExampleLocalTimeZone_GetOneZone_sanFrancisco() {
	z := localtimezone.NewLocalTimeZone()
	// Loading zone for San Francisco, CA
	p := localtimezone.Point{
		Lon: -122.4194, Lat: 37.7749,
	}
	zone, err := z.GetOneZone(p)
	if err != nil {
		panic(err)
	}
	fmt.Println(zone)
	// Output: America/Los_Angeles
}

func ExampleLocalTimeZone_GetOneZone_alaskaPanhandle() {
	z := localtimezone.NewLocalTimeZone()
	// Loading zone for the Alaska panhandle (overlaps America/Sitka and America/Vancouver)
	p := localtimezone.Point{
		Lon: -132.783555, Lat: 54.554439,
	}
	zone, err := z.GetOneZone(p)
	if err != nil {
		panic(err)
	}
	fmt.Println(zone)
	// Output: America/Sitka
}
