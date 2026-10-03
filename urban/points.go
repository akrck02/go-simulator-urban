package urban

import "math"

// Point is a position on the ground plane (x, z), in world units (km).
type Point struct{ X, Z float64 }

// Add returns the sum of two points taken as vectors.
func (point Point) Add(other Point) Point { return Point{point.X + other.X, point.Z + other.Z} }

// Subtract returns point − other.
func (point Point) Subtract(other Point) Point { return Point{point.X - other.X, point.Z - other.Z} }

// Scale multiplies the point taken as a vector.
func (point Point) Scale(factor float64) Point { return Point{point.X * factor, point.Z * factor} }

// Length returns the length of the point taken as a vector.
func (point Point) Length() float64 { return math.Hypot(point.X, point.Z) }

// DistanceTo returns the distance between two points.
func (point Point) DistanceTo(other Point) float64 { return point.Subtract(other).Length() }

// Dot returns the dot product of two vectors.
func (point Point) Dot(other Point) float64 { return point.X*other.X + point.Z*other.Z }

// AngleTo returns the direction from this point to another (radians).
func (point Point) AngleTo(other Point) float64 { return math.Atan2(other.Z-point.Z, other.X-point.X) }

// Towards returns the point at a distance in a direction (radians).
func (point Point) Towards(angle, distance float64) Point {
	return Point{point.X + distance*math.Cos(angle), point.Z + distance*math.Sin(angle)}
}

// Segment is a straight piece between two points.
type Segment struct{ From, To Point }

// Length returns the length of the segment.
func (segment Segment) Length() float64 { return segment.From.DistanceTo(segment.To) }

// Middle returns the middle point.
func (segment Segment) Middle() Point { return segment.At(0.5) }

// At returns the point at a share (0..1) of the way.
func (segment Segment) At(share float64) Point {
	return segment.From.Add(segment.To.Subtract(segment.From).Scale(share))
}

// Project returns how far along the segment the closest point to another lies (0..length).
func (segment Segment) Project(point Point) float64 {
	direction := segment.To.Subtract(segment.From)
	squared := direction.Dot(direction)
	if squared == 0 {
		return 0
	}
	share := clamp(point.Subtract(segment.From).Dot(direction)/squared, 0, 1)
	return share * math.Sqrt(squared)
}

// Closest returns the point of the segment closest to another.
func (segment Segment) Closest(point Point) Point {
	length := segment.Length()
	if length == 0 {
		return segment.From
	}
	return segment.At(segment.Project(point) / length)
}

// DistanceTo returns the distance from the segment to a point.
func (segment Segment) DistanceTo(point Point) float64 {
	return segment.Closest(point).DistanceTo(point)
}

// Crossing returns where two segments cross, if they do (touching ends count).
func (segment Segment) Crossing(other Segment) (Point, bool) {
	direction, otherDirection := segment.To.Subtract(segment.From), other.To.Subtract(other.From)
	denominator := direction.X*otherDirection.Z - direction.Z*otherDirection.X
	if math.Abs(denominator) < 1e-15 {
		return Point{}, false // parallel or degenerate
	}
	offset := other.From.Subtract(segment.From)
	share := (offset.X*otherDirection.Z - offset.Z*otherDirection.X) / denominator
	otherShare := (offset.X*direction.Z - offset.Z*direction.X) / denominator
	const tolerance = 1e-12
	if share < -tolerance || share > 1+tolerance || otherShare < -tolerance || otherShare > 1+tolerance {
		return Point{}, false
	}
	return segment.At(share), true
}

// angleDifference returns the signed difference between two angles in (−π, π], like math.remainder.
func angleDifference(first, second float64) float64 { return math.Remainder(first-second, 2*math.Pi) }
