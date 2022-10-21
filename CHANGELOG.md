CHANGELOG
=========

v2.1.0
------

 - Add `NewMockLocalTimeZone` which always returns `"America/Los_Angeles"`


v2.0.4
------

 - Move shape file into an embedded gzip
 - Update readme
 - Update dependencies


v2.0.3
------

 - Fix godocs
 - Refactor tzshapefilegen


v2.0.2
------

 - Fix go.mod and imports to use v2


v2.0.1
------

 - Defer loading logic on startup
 - Use a faster json library


v2.0.0
------

 - Refactor library to be based around a client rather than exported functions
 - Add the ability to update tzdata to the latest version without having to pass flags
 - Add timezone tests for all cities with >1M population
 - Optimizations


v1.0.0
------
 - Initial release of mostly https://github.com/ugjka/go-tz
 - Updated to tzdata 2021c
 - Updated documentation
 - Additional tests
