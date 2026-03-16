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

Uses timezone boundary data from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder/), indexed with [H3](https://h3geo.org/) hexagonal cells for fast lookups.

## Features

- The timezone data is embedded in the build binary
- `GetZone()` returns all timezones at a location; `GetOneZone()` returns a single result
- You can load custom GeoJSON data for alternative data sources
- Thread-safe for concurrent lookups
- Lookups are purely in-memory. Uses ~8MB of RAM.

### Benchmarks

```
go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/albertyw/localtimezone/v3
cpu: AMD Ryzen 9 7900X 12-Core Processor
BenchmarkGetZone/GetZone_on_large_cities-24              1000000              1071 ns/op             112 B/op         11 allocs/op
BenchmarkGetZone/GetOneZone_on_large_cities-24           1273423               940.3 ns/op            90 B/op          8 allocs/op
BenchmarkZones/test_cases-24                              239356              4605 ns/op            1023 B/op        109 allocs/op
BenchmarkClientInit/main_client-24                            55          21337062 ns/op        31011069 B/op        829 allocs/op
BenchmarkClientInit/mock_client-24                        109257             11199 ns/op           43152 B/op         19 allocs/op
PASS
ok      github.com/albertyw/localtimezone/v3    6.038s
```

Lookups take ~1 microsecond; client initialization takes ~18ms.

## Limitations

- H3 hexagonal discretization (resolution 7, ~5.16 km² per cell) may have reduced accuracy near timezone borders
- Points in international waters or disputed territories return the nearest timezone

## Updating data

To update to the latest timezone data:

```bash
make generate
```

The data comes from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder). Check the releases page for the latest version.

## Licenses

The code used to lookup the timezone for a location is licensed under the [MIT License](https://opensource.org/licenses/MIT).

The timezone boundary data is licensed under the [Open Data Commons Open Database License (ODbL)](https://opendatacommons.org/licenses/odbl/).
