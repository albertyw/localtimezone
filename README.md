# github.com/albertyw/localtimezone

LatLong conversion to time zone

[![Build Status](https://drone.albertyw.com/api/badges/albertyw/localtimezone/status.svg)](https://drone.albertyw.com/albertyw/localtimezone)
[![GoDoc](https://godoc.org/github.com/albertyw/localtimezone?status.svg)](http://godoc.org/github.com/albertyw/localtimezone)
[![Go Report Card](https://goreportcard.com/badge/github.com/albertyw/localtimezone)](https://goreportcard.com/report/github.com/albertyw/localtimezone)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

lookup timezone for a given location

## Usage / Example

```go
import "github.com/albertyw/localtimezone"

// Loading Zone for Line Islands, Kiritimati
zone, err := tz.GetZone(tz.Point{
    Lon: -157.21328, Lat: 1.74294,
})
if err != nil {
    panic(err)
}
fmt.Println(zone[0])
```

Uses simplified shapefile from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder/)

GeoJson Simplification done with [mapshaper](http://mapshaper.org/)

## Features

- The timezone shapefile is embedded in the build binary
- Supports overlapping zones
- You can load your own geojson shapefile if you want
- Sub millisecond lookup even on old hardware

## Problems

- Shapefile is simplified using a lossy method so it may be innacurate along the borders
- This is purely in-memory. Uses ~50MB of ram

## Updating data
Get the most current timezone release version at https://github.com/evansiroky/timezone-boundary-builder/tags

```bash
go run tzshapefilegen/main.go -release [2021c]
```

## Licenses

The code used to lookup the timezone for a location is licensed under the [MIT License](https://opensource.org/licenses/MIT).

The data in timezone shapefile is licensed under the [Open Data Commons Open Database License (ODbL)](http://opendatacommons.org/licenses/odbl/).
