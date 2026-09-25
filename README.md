# github.com/albertyw/localtimezone

[![Build Status](https://drone.albertyw.com/api/badges/albertyw/localtimezone/status.svg)](https://drone.albertyw.com/albertyw/localtimezone)
[![Go Reference](https://pkg.go.dev/badge/github.com/albertyw/localtimezone/v4.svg)](https://pkg.go.dev/github.com/albertyw/localtimezone/v4)
[![Maintainability](https://qlty.sh/gh/albertyw/projects/localtimezone/maintainability.svg)](https://qlty.sh/gh/albertyw/projects/localtimezone)
[![Code Coverage](https://qlty.sh/gh/albertyw/projects/localtimezone/coverage.svg)](https://qlty.sh/gh/albertyw/projects/localtimezone)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

Provides timezone lookup for geographic coordinates.
Based on [github.com/ugjka/go-tz](https://github.com/ugjka/go-tz).

## Installation

```bash
go get github.com/albertyw/localtimezone/v4
```

## Usage / Example

See [example_test.go](https://github.com/albertyw/localtimezone/blob/master/example_test.go) for more examples.

```go
import localtimezone "github.com/albertyw/localtimezone/v4"

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

### Limitations

- H3 hexagonal discretization (resolution 7, ~5.16 km² per cell) may have reduced accuracy near timezone borders
- Points in international waters or disputed territories return the nearest timezone

### Benchmarks

```
go test -bench=. -benchmem
goos: linux
goarch: amd64
pkg: github.com/albertyw/localtimezone/v4
cpu: AMD Ryzen 9 7900X 12-Core Processor
BenchmarkGetZone/GetZone_on_large_cities-24              1637100               736.2 ns/op            40 B/op          3 allocs/op
BenchmarkGetZone/GetOneZone_on_large_cities-24           1982846               604.9 ns/op            24 B/op          2 allocs/op
BenchmarkZones/test_cases-24                              807022              1474 ns/op             170 B/op          4 allocs/op
BenchmarkClientInit/main_client-24                           205           5473409 ns/op        18004375 B/op        425 allocs/op
BenchmarkClientInit/mock_client-24                        681598              1676 ns/op            2688 B/op          7 allocs/op
PASS
ok      github.com/albertyw/localtimezone/v4    6.113s
```

Lookups take under a microsecond; client initialization takes ~5ms.

```
go test -bench=BenchmarkClientMemory -benchmem -memprofile memprofile.out
goos: linux
goarch: amd64
pkg: github.com/albertyw/localtimezone/v4
cpu: AMD Ryzen 9 7900X 12-Core Processor
BenchmarkClientMemory/main_client-24                 151           9009555 retained-B/op        18004371 B/op        425 allocs/op
BenchmarkClientMemory/mock_client-24                1245              1398 retained-B/op            2691 B/op          7 allocs/op
PASS
ok      github.com/albertyw/localtimezone/v4    2.501s
```

A client retains ~9MB of heap; the other half of the ~18MB allocated during initialization is a decompression buffer that is freed afterwards.

## Development

```bash
# To update to the latest timezone data
make generate

# To run tests
make test
make race

# To run benchmarks
make benchmark
make benchmark-memory

# To render the data as an interactive world map at tzmap/map.html
make map
```

The map rasterizes the world at 0.1° spacing with `GetOneZone`. Hovering a region shows its timezone, or the computed nautical UTC offset where no official timezone applies, and clicking opens the zone's Wikipedia page.

The data comes from [timezone-boundary-builder](https://github.com/evansiroky/timezone-boundary-builder). Check the releases page for the latest version.

## Architecture

At build time, timezone polygon boundaries from timezone-boundary-builder are converted to [H3](https://h3geo.org/) hexagonal cells at resolution 7 (~5.16 km² per cell). Cells covering a uniform timezone region are compacted into coarser-resolution parent cells, shrinking the dataset significantly. The result is serialized into a custom binary format (`H3TZ`), compressed with [S2](https://github.com/klauspost/compress), and embedded directly into the Go binary via `//go:embed`.

At runtime, a lookup converts the input coordinates to an H3 cell ID, then binary-searches a sorted array of cell→timezone entries. If the exact cell is absent (due to compaction), the search walks up the H3 hierarchy to coarser resolutions. Parent cells are derived with bit operations in Go rather than through H3's C library, so unless a lookup falls through to the neighbor search, converting the coordinates is its only cgo call. An H3 index stores its resolution above its other fields, so a parent always sorts before its children; each coarser search therefore only covers the entries below where the previous search ended. `GetOneZone()` stops at the first match without building a result slice, while `GetZone()` collects every overlapping zone. Points in international waters fall back to an expanding ring search over neighboring cells, then to a nautical zone derived from longitude.

```
Build time:
  GeoJSON polygons
    └─► H3 PolygonToCells (resolution 7)
          └─► CompactCells
                └─► H3TZ binary → S2 compress → //go:embed

Runtime (GetZone / GetOneZone):
  (lat, lon)
    └─► H3 cell ID                              (cgo)
          └─► binary search sorted cells[]
                └─► walk parent resolutions     (compaction)
                    │   parents computed in Go; each search is
                    │   bounded by the previous one's position
                    └─► GridDisk neighbor fallback
                          └─► nautical zone fallback
```

## Licenses

The code used to lookup the timezone for a location is licensed under the [MIT License](https://opensource.org/licenses/MIT).

The timezone boundary data is licensed under the [Open Data Commons Open Database License (ODbL)](https://opendatacommons.org/licenses/odbl/).
