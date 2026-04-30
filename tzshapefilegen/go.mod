module github.com/albertyw/localtimezone/v4/tzshapefilegen

go 1.24

require (
	github.com/albertyw/localtimezone/v4 v4.0.1
	github.com/klauspost/compress v1.18.5
	github.com/paulmach/orb v0.12.0
	github.com/uber/h3-go/v4 v4.4.1
)

require go.mongodb.org/mongo-driver v1.11.4 // indirect

replace github.com/albertyw/localtimezone/v4 => ../
