# github.com/albertyw/localtimezone

[![Build Status](https://drone.albertyw.com/api/badges/albertyw/localtimezone/status.svg)](https://drone.albertyw.com/albertyw/localtimezone)
[![Go Reference](https://pkg.go.dev/badge/github.com/albertyw/localtimezone/v3.svg)](https://pkg.go.dev/github.com/albertyw/localtimezone/v3)
[![Go Report Card](https://goreportcard.com/badge/github.com/albertyw/localtimezone/v3)](https://goreportcard.com/report/github.com/albertyw/localtimezone/v3)
[![Maintainability](https://qlty.sh/gh/albertyw/projects/localtimezone/maintainability.svg)](https://qlty.sh/gh/albertyw/projects/localtimezone)
[![Code Coverage](https://qlty.sh/gh/albertyw/projects/localtimezone/coverage.svg)](https://qlty.sh/gh/albertyw/projects/localtimezone)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Provides timezone lookup for geographic coordinates.
Based on [github.com/ugjka/go-tz](https://github.com/ugjka/go-tz).

## Installation

```bash
go get github.com/albertyw/localtimezone/v3
```

## Usage / Example

```go
import localtimezone "github.com/albertyw/localtimezone/v3"

// Loading Zone for Line Islands, Kiritimati
tz, err := localtimezone.NewLocalTimeZone()
if err != nil {
    panic(err)
}
zone, err := tz.GetZone(localtimezone.Point{
    Lon: -157.21328, Lat: 1.74294,
})
if err != nil {
    panic(err)
}
fmt.Println(zone[0])
```

Note: `GetZone()` may return an error only for out-of-range coordinates; it returns the nearest timezone for all valid locations.

Uses simplified shapefile from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder/)

Built with [orb](https://github.com/paulmach/orb) for high-performance geometry handling and WKB serialization.

## Features

- The timezone shapefile is embedded in the build binary
- `GetZone()` returns all timezones at a location; `GetOneZone()` returns a single result
- You can load a custom GeoJSON shapefile for alternative data sources
- Thread-safe for concurrent lookups
- Lookups are purely in-memory. Uses ~8MB of RAM.

### Benchmarks

```
go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/albertyw/localtimezone/v3
cpu: AMD Ryzen 9 7900X 12-Core Processor
BenchmarkGetZone/GetZone_on_large_cities-24                41272             29076 ns/op              16 B/op          1 allocs/op
BenchmarkGetZone/GetOneZone_on_large_cities-24             54379             21986 ns/op              16 B/op          1 allocs/op
BenchmarkZones/polygon_centers-24                          75387             15807 ns/op              16 B/op          1 allocs/op
BenchmarkZones/test_cases-24                              170304              6982 ns/op              18 B/op          1 allocs/op
BenchmarkClientInit/main_client-24                           153           7821843 ns/op         7265567 B/op       9161 allocs/op
BenchmarkClientInit/mock_client-24                        113492             10889 ns/op           37880 B/op         30 allocs/op
PASS
ok      github.com/albertyw/localtimezone/v3    7.312s
```

Lookups take ~6-30 microseconds depending on location complexity; client initialization takes ~14ms.

## Limitations

- Shapefile uses simplified geometries (Visvalingam simplification with ~89m threshold) that may have reduced accuracy near borders
- Points in international waters or disputed territories return the nearest timezone

## Updating data

To update to the latest timezone data:

```bash
make generate
```

The data comes from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder). Check the releases page for the latest version.

## Licenses

The code used to lookup the timezone for a location is licensed under the [MIT License](https://opensource.org/licenses/MIT).

The data in timezone shapefile is licensed under the [Open Data Commons Open Database License (ODbL)](https://opendatacommons.org/licenses/odbl/).
