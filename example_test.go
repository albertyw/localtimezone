package localtimezone_test

import (
	"fmt"

	"github.com/albertyw/localtimezone"
)

func ExampleGetZone() {
	z := localtimezone.NewLocalTimeZone()
	// Loading Zone for Line Islands, Kiritimati
	p := localtimezone.Point{
		Lon: -157.21328, Lat: 1.74294,
	}
	zone, err := z.GetZone(p)
	if err != nil {
		panic(err)
	}
	fmt.Println(zone[0])
	// Output: Pacific/Kiritimati
}
