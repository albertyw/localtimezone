CHANGELOG
=========

v3.0.2 (2023-01-01)
-------------------

 - Performance optimizations
 - Testing/benchmarking improvements


v3.0.1 (2022-12-02)
-------------------

 - Update tzdata from 2022f to 2022g.  Full changelog at https://github.com/evansiroky/timezone-boundary-builder/releases/tag/2022g
 - Check all returned errors with `errcheck`
 - Continually check for tzdata freshness
 - Update CI configuration


v3.0.0 (2022-11-12)
-------------------

 - Remove all geo-related types including `FeatureCollection`, `Feature`, `Geometry`
 - Remove unused `ErrNoZoneFound`
 - Remove all internal geo logic and use github.com/paulmach/orb (previously only used for generating map data)
 - Improved performance through parallelization


v2.1.4 (2022-10-31)
-------------------

 - Update tzdata from 2022d to 2022f.  Full changelog at https://github.com/evansiroky/timezone-boundary-builder/releases/tag/2022f
 - Update dependencies


v2.1.3 (2022-10-30)
-------------------

 - Update tzdata from 2022b to 2022d.  Full changelog at https://github.com/evansiroky/timezone-boundary-builder/releases/tag/2022d
 - Update test coverage calculation
 - Improve performance benchmarking


v2.1.2 (2022-10-24)
-------------------

 - Update from tzdata 2021c to 2022b.  Full changelog at https://github.com/evansiroky/timezone-boundary-builder/releases/tag/2022b
   This update changes several timezone borders.
   This update also renames the `Europe/Kiev` timezone to `Europe/Kyiv`.  Note that **this may be backwards incompatible** depending on how you are using timezone data.


v2.1.1 (2022-10-22)
-------------------

 - Add a `MockTimeZone` which is the timezone always returned by the `NewMockLocalTimeZone` client
 - Significant client loading speedup by optimizing parsing of geojson data
 - Replace geojson dependency on js mapshaper with go orb package


v2.1.0 (2022-10-20)
-------------------

 - Add `NewMockLocalTimeZone` which always returns `"America/Los_Angeles"`


v2.0.4 (2022-10-18)
-------------------

 - Move shape file into an embedded gzip
 - Update readme
 - Update dependencies


v2.0.3 (2022-10-01)
-------------------

 - Fix godocs
 - Refactor tzshapefilegen


v2.0.2 (2022-09-20)
-------------------

 - Fix go.mod and imports to use v2


v2.0.1 (2022-09-14)
-------------------

 - Defer loading logic on startup
 - Use a faster json library


v2.0.0 (2022-09-12)
-------------------

 - Refactor library to be based around a client rather than exported functions
 - Add the ability to update tzdata to the latest version without having to pass flags
 - Add timezone tests for all cities with >1M population
 - Optimizations


v1.0.0 (2022-09-05)
-------------------
 - Initial release of mostly https://github.com/ugjka/go-tz
 - Updated to tzdata 2021c
 - Updated documentation
 - Additional tests
