package urban

import (
	"github.com/twpayne/go-geos"
)

// Geometry on the ground plane goes through GEOS (the engine shapely uses), with x as GEOS's x and
// z as GEOS's y.

func pointGeometry(point Point) *geos.Geom { return geos.NewPointFromXY(point.X, point.Z) }

func segmentGeometry(segment Segment) *geos.Geom {
	return geos.NewLineString([][]float64{{segment.From.X, segment.From.Z}, {segment.To.X, segment.To.Z}})
}

func lineGeometry(points []Point) *geos.Geom {
	coordinates := make([][]float64, 0, len(points))
	for _, point := range points {
		coordinates = append(coordinates, []float64{point.X, point.Z})
	}
	return geos.NewLineString(coordinates)
}

// polygonGeometry closes the ring itself.
func polygonGeometry(points []Point) *geos.Geom {
	ring := make([][]float64, 0, len(points)+1)
	for _, point := range points {
		ring = append(ring, []float64{point.X, point.Z})
	}
	ring = append(ring, []float64{points[0].X, points[0].Z})
	return geos.NewPolygon([][][]float64{ring})
}

func emptyGeometry() *geos.Geom { return geos.NewEmptyPolygon() }

// unionOf joins geometries into one (they are copied: the originals stay usable).
func unionOf(geometries ...*geos.Geom) *geos.Geom {
	copies := make([]*geos.Geom, 0, len(geometries))
	for _, geometry := range geometries {
		if geometry != nil && !geometry.IsEmpty() {
			copies = append(copies, geometry.Clone())
		}
	}
	if len(copies) == 0 {
		return emptyGeometry()
	}
	return geos.NewCollection(geos.TypeIDGeometryCollection, copies).UnaryUnion()
}

// partsOf lists the pieces of a geometry: itself, or the members of a multi-geometry or collection.
func partsOf(geometry *geos.Geom) []*geos.Geom {
	if geometry == nil || geometry.IsEmpty() {
		return nil
	}
	switch geometry.TypeID() {
	case geos.TypeIDMultiPoint, geos.TypeIDMultiLineString, geos.TypeIDMultiPolygon, geos.TypeIDGeometryCollection:
		parts := make([]*geos.Geom, 0, geometry.NumGeometries())
		for index := range geometry.NumGeometries() {
			parts = append(parts, partsOf(geometry.Geometry(index).Clone())...)
		}
		return parts
	}
	return []*geos.Geom{geometry}
}

// polygonsOf lists the polygons in a geometry.
func polygonsOf(geometry *geos.Geom) []*geos.Geom {
	var polygons []*geos.Geom
	for _, part := range partsOf(geometry) {
		if part.TypeID() == geos.TypeIDPolygon {
			polygons = append(polygons, part)
		}
	}
	return polygons
}

// largestPolygon returns the polygon with the most area, or nil.
func largestPolygon(geometry *geos.Geom) *geos.Geom {
	var largest *geos.Geom
	for _, polygon := range polygonsOf(geometry) {
		if largest == nil || polygon.Area() > largest.Area() {
			largest = polygon
		}
	}
	return largest
}

// linesOf lists the line strings in a geometry.
func linesOf(geometry *geos.Geom) []*geos.Geom {
	var lines []*geos.Geom
	for _, part := range partsOf(geometry) {
		switch part.TypeID() {
		case geos.TypeIDLineString:
			lines = append(lines, part)
		case geos.TypeIDLinearRing:
			lines = append(lines, geos.NewLineString(part.CoordSeq().ToCoords()))
		}
	}
	return lines
}

// pointsOf lists the points in a geometry.
func pointsOf(geometry *geos.Geom) []Point {
	var points []Point
	for _, part := range partsOf(geometry) {
		if part.TypeID() == geos.TypeIDPoint {
			points = append(points, Point{part.X(), part.Y()})
		}
	}
	return points
}

func centroidOf(geometry *geos.Geom) Point {
	centroid := geometry.Centroid()
	return Point{centroid.X(), centroid.Y()}
}

func boundsOf(geometry *geos.Geom) (Point, Point) {
	box := geometry.Bounds()
	return Point{box.MinX, box.MinY}, Point{box.MaxX, box.MaxY}
}

// coordinatesOf returns the points of a line string.
func coordinatesOf(line *geos.Geom) []Point {
	coordinates := line.CoordSeq().ToCoords()
	points := make([]Point, 0, len(coordinates))
	for _, coordinate := range coordinates {
		points = append(points, Point{coordinate[0], coordinate[1]})
	}
	return points
}

// outlineOf returns a polygon's outer ring as ground coordinates, without repeating the first point.
func outlineOf(polygon *geos.Geom) [][2]float32 { return ringOf(polygon.ExteriorRing()) }

// ringOf returns a ring as ground coordinates, without repeating the first point.
func ringOf(linearRing *geos.Geom) [][2]float32 {
	ring := linearRing.CoordSeq().ToCoords()
	outline := make([][2]float32, 0, len(ring))
	for index, coordinate := range ring {
		if index == len(ring)-1 && len(ring) > 1 {
			break
		}
		outline = append(outline, [2]float32{float32(coordinate[0]), float32(coordinate[1])})
	}
	return outline
}

// lineCoordinatesOf returns a line string as ground coordinates.
func lineCoordinatesOf(line *geos.Geom) [][2]float32 {
	points := coordinatesOf(line)
	coordinates := make([][2]float32, 0, len(points))
	for _, point := range points {
		coordinates = append(coordinates, [2]float32{float32(point.X), float32(point.Z)})
	}
	return coordinates
}

// valid repairs a geometry that GEOS considers invalid.
func valid(geometry *geos.Geom) *geos.Geom {
	if geometry.IsValid() {
		return geometry
	}
	return geometry.MakeValid()
}

// safeDifference subtracts, repairing the subtrahend if GEOS fails on it (like buffer(0) in shapely).
func safeDifference(geometry, subtrahend *geos.Geom) (result *geos.Geom, repaired *geos.Geom) {
	defer func() {
		if recovered := recover(); recovered != nil {
			repaired = subtrahend.Buffer(0, 8)
			result = geometry.Difference(repaired)
		}
	}()
	return geometry.Difference(subtrahend), subtrahend
}
