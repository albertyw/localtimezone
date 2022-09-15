package localtimezone

import (
	"github.com/goccy/go-json"
)

// FeatureCollection is a set of Features
type FeatureCollection struct {
	Features []*Feature
}

// Feature maps a Geometry with TZids
type Feature struct {
	Geometry   Geometry
	Properties struct {
		Tzid string
	}
}

// Geometry represents a set of points that draw a geographic geometry
type Geometry struct {
	Coordinates   [][]Point
	BoundingBoxes [][]Point
}

type jPolyTypeType struct {
	Type       string
	Geometries []*Geometry
}

type jPolygonType struct {
	Coordinates [][][]float64
}

type jMultiPolygonType struct {
	Coordinates [][][][]float64
}

// UnmarshalJSON parses json data into a geometry
func (g *Geometry) UnmarshalJSON(data []byte) (err error) {
	var jPolyType jPolyTypeType
	if err := json.Unmarshal(data, &jPolyType); err != nil {
		return err
	}

	if jPolyType.Type == "Polygon" {
		var jPolygon jPolygonType
		if err := json.Unmarshal(data, &jPolygon); err != nil {
			return err
		}
		pol := make([]Point, len(jPolygon.Coordinates[0]))
		for i, v := range jPolygon.Coordinates[0] {
			pol[i].Lon = v[0]
			pol[i].Lat = v[1]
		}
		b := getBoundingBox(pol)
		g.BoundingBoxes = append(g.BoundingBoxes, b)
		g.Coordinates = append(g.Coordinates, pol)
		return nil
	}

	if jPolyType.Type == "MultiPolygon" {
		var jMultiPolygon jMultiPolygonType
		if err := json.Unmarshal(data, &jMultiPolygon); err != nil {
			return err
		}
		g.BoundingBoxes = make([][]Point, len(jMultiPolygon.Coordinates))
		g.Coordinates = make([][]Point, len(jMultiPolygon.Coordinates))
		for j, poly := range jMultiPolygon.Coordinates {
			pol := make([]Point, len(poly[0]))
			for i, v := range poly[0] {
				pol[i].Lon = v[0]
				pol[i].Lat = v[1]
			}
			b := getBoundingBox(pol)
			g.BoundingBoxes[j] = b
			g.Coordinates[j] = pol
		}
		return nil
	}
	return nil
}
