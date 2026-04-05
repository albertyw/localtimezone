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

See [example_test.go](https://github.com/albertyw/localtimezone/blob/master/example_test.go) for more examples.

```go
import localtimezone "github.com/albertyw/localtimezone/v3"

tz := localtimezone.NewLocalTimeZone()

// For most use cases, use GetOneZone to get any timezone for a location
zone, err := tz.GetOneZone(localtimezone.Point{
    Lon: -122.4194, Lat: 37.7749,
})
if err != nil {
    panic(err)
}
fmt.Println(zone)
// Output: America/Los_Angeles

// Some timezones overlap and may return multiple zones
zone, err := tz.GetZone(localtimezone.Point{
    Lon: -132.783555, Lat: 54.554439,
})
if err != nil {
    panic(err)
}
for _, zone:= range zones {
    fmt.Println(zone)
}
// Output:
// America/Sitka
// America/Vancouver
```

Note: `GetZone()` may return an error only for out-of-range coordinates; it returns the nearest timezone for all valid locations.

Uses timezone boundary data from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder/), indexed with [H3](https://h3geo.org/) hexagonal cells for fast lookups.

## Features

- The timezone data is embedded in the build binary
- `GetZone()` returns all timezones at a location; `GetOneZone()` returns a single result
- Thread-safe for concurrent lookups
- Lookups are purely in-memory. Uses ~17MB of RAM.

### Benchmarks

```
go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/albertyw/localtimezone/v3
cpu: AMD Ryzen 9 7900X 12-Core Processor
BenchmarkGetZone/GetZone_on_large_cities-24               989595              1205 ns/op             116 B/op         11 allocs/op
BenchmarkGetZone/GetOneZone_on_large_cities-24           1000000              1006 ns/op              86 B/op          7 allocs/op
BenchmarkZones/test_cases-24                              237181              4559 ns/op            1025 B/op        110 allocs/op
BenchmarkClientInit/main_client-24                           247           4772720 ns/op        17947054 B/op        425 allocs/op
BenchmarkClientInit/mock_client-24                        735993              1458 ns/op            2688 B/op          7 allocs/op
PASS
ok      github.com/albertyw/localtimezone/v3    5.662s
```

Lookups take ~1 microsecond; client initialization takes ~5ms.

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
