module github.com/albertyw/localtimezone/v4/tzshapefilegen

go 1.25.0

require (
	github.com/albertyw/localtimezone/v4 v4.0.2
	github.com/klauspost/compress v1.20.0
	github.com/paulmach/orb v0.13.0
	github.com/uber/h3-go/v4 v4.5.0
)

require go.mongodb.org/mongo-driver/v2 v2.9.1 // indirect

replace github.com/albertyw/localtimezone/v4 => ../
