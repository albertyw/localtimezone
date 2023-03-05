package main

import (
	"fmt"

	localtimezone "github.com/albertyw/localtimezone/v3"
)

func main() {
	z, err := localtimezone.NewLocalTimeZone()
	if err != nil {
		panic(err)
	}
	zone, err := z.GetZone(localtimezone.Point{
		// Lon: -157.21328, Lat: 1.74294, // Pacific/Kiritimati
		// Lon: -57.637517, Lat: -25.335772, // America/Asuncion
		Lon: -57.681572, Lat: -25.351069, // America/Argentina/Cordoba

	})
	if err != nil {
		panic(err)
	}
	fmt.Println(zone[0])
}
