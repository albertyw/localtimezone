package main

import (
	"fmt"

	"github.com/albertyw/localtimezone"
)

func main() {
	zone, err := localtimezone.GetZone(localtimezone.Point{
		Lon: -157.21328, Lat: 1.74294,
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(zone[0])
}
