module github.com/albertyw/localtimezone/v3/tzshapefilegen

go 1.24

require (
	github.com/albertyw/localtimezone/v3 v3.0.0
	github.com/klauspost/compress v1.18.4
	github.com/paulmach/orb v0.12.0
	github.com/uber/h3-go/v4 v4.4.0
)

require go.mongodb.org/mongo-driver v1.11.4 // indirect

replace github.com/albertyw/localtimezone/v3 => ../
