module github.com/albertyw/localtimezone/v4/tzshapefilegen

go 1.24

require (
	github.com/albertyw/localtimezone/v4 v4.0.2
	github.com/klauspost/compress v1.19.0
	github.com/paulmach/orb v0.12.0
	github.com/uber/h3-go/v4 v4.5.0
)

require go.mongodb.org/mongo-driver v1.17.7 // indirect

replace github.com/albertyw/localtimezone/v4 => ../
