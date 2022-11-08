package localtimezone

import (
	"math"

	"github.com/paulmach/orb"
)

type polygon []Point

func (p polygon) isClosed() bool {
	return len(p) >= 3
}

// Returns whether or not the current Polygon contains the passed in Point.
func (p polygon) contains(point orb.Point) bool {
	if !p.isClosed() {
		return false
	}

	start := len(p) - 1
	end := 0

	contains := intersectsWithRaycast(point, &p[start], &p[end])

	for i := 1; i < len(p); i++ {
		if intersectsWithRaycast(point, &p[i-1], &p[i]) {
			contains = !contains
		}
	}

	return contains
}

// https://rosettacode.org/wiki/Ray-casting_algorithm#Go
func intersectsWithRaycast(point orb.Point, start, end *Point) bool {
	if start.Lat > end.Lat {
		start, end = end, start
	}
	for point[1] == start.Lat || point[1] == end.Lat {
		point[1] = math.Nextafter(point[1], math.Inf(1))
	}
	if point[1] < start.Lat || point[1] > end.Lat {
		return false
	}
	if start.Lon > end.Lon {
		if point[0] > start.Lon {
			return false
		}
		if point[0] < end.Lon {
			return true
		}
	} else {
		if point[0] > end.Lon {
			return false
		}
		if point[0] < start.Lon {
			return true
		}
	}
	return (point[1]-start.Lat)/(point[0]-start.Lon) >= (end.Lat-start.Lat)/(end.Lon-start.Lon)
}
