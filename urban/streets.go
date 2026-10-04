package urban

import (
	"math"
	"math/rand/v2"
	"sort"
)

// New streets follow the stochastic expansion of Weber, Müller, Wonka and Gross (2009): a node is
// chosen with a probability that falls with the distance to the growth centres; the street leaves
// straight on, sideways or into the missing direction depending on how many streets the node
// already has, bends towards a tensor field (Chen et al. 2008: contour lines on slopes, radial
// around the centres, a grid in planned districts) and must pass Weber's legality tests.

const (
	steepForStreets = 0.15 // above this slope a street must follow the contour line
	shortestStreet  = 0.012
	closingFactor   = 1.5
	narrowestAngle  = 30 * math.Pi / 180
	longestBridge   = 0.03
	bridgeChance    = 0.35
	gateReach       = 0.012
	maximumFailures = 4
)

// expand covers the year's deficit: plots along free frontages first and, when none are left,
// new streets, up to how many streets a city can open in a year.
func (city *City) expand() {
	attempts, streetsThisYear := 0, 0
	streetLimit := 6 + int(0.04*city.streetLength()/defaultSegment) // grows with the town, not with how finely it is cut
	for city.Deficit > 0 && attempts < 40 {
		attempts++
		if streetsThisYear < streetLimit && city.bestFreeFrontage() < city.Scenario.OpenStreetsBelow {
			streetsThisYear += city.growStreets(min(streetLimit-streetsThisYear, 3+int(city.Deficit/20000)))
		}
		gained := city.buildFrontages(city.Deficit)
		city.Deficit -= gained
		if gained > 0 {
			continue
		}
		if streetsThisYear >= streetLimit {
			return
		}
		opened := city.growStreets(min(streetLimit-streetsThisYear, 3+int(city.Deficit/20000)))
		streetsThisYear += opened
		if opened == 0 {
			return
		}
	}
}

const defaultSegment = 0.055 // the street piece the yearly limit was calibrated with

// streetLength is the total length of the street network.
func (city *City) streetLength() float64 {
	total := 0.0
	for _, edge := range city.Network.Edges {
		total += city.Network.Segment(edge).Length()
	}
	return total
}

// growStreets tries to open some streets from nodes chosen by weight; it returns how many opened.
func (city *City) growStreets(wanted int) int {
	candidates := city.streetCandidates()
	if len(candidates) == 0 {
		return 0
	}
	city.refreshYearlyWeights()
	weights := make([]float64, len(candidates))
	for index, node := range candidates {
		weights[index] = city.nodeWeight(node) + 1e-9
	}
	opened := 0
	for _, node := range weightedChoices(city.random, candidates, weights, min(len(candidates), wanted*3)) {
		if opened >= wanted {
			break
		}
		if city.tryStreet(node) {
			opened++
		} else {
			city.Network.failures[node]++
		}
	}
	return opened
}

func (city *City) streetCandidates() []int {
	var candidates []int
	for node, point := range city.Network.Nodes {
		degree := city.Network.Degree(node)
		if degree >= 1 && degree <= 3 && city.Network.failures[node] < maximumFailures && city.Window.Contains(pointGeometry(point)) {
			candidates = append(candidates, node)
		}
	}
	return candidates
}

// weightedChoices draws some items with replacement, each with probability ∝ its weight.
func weightedChoices(random *rand.Rand, items []int, weights []float64, count int) []int {
	cumulative := make([]float64, len(weights))
	sum := 0.0
	for index, weight := range weights {
		sum += weight
		cumulative[index] = sum
	}
	chosen := make([]int, 0, count)
	for range count {
		target := random.Float64() * sum
		index := sort.SearchFloat64s(cumulative, target)
		chosen = append(chosen, items[min(index, len(items)-1)])
	}
	return chosen
}

// nodeWeight is how likely a street opens from a node this year: near the centres, inside the
// wall, and at the edge of what is built rather than between finished houses (exploration,
// Strano et al. 2012) or in open country.
func (city *City) nodeWeight(node int) float64 {
	if weight, known := city.nodeWeights[node]; known {
		return weight
	}
	point := city.Network.Nodes[node]
	weight := city.closeness(point, city.growthRadius(), total)
	if city.CurrentWall() != nil {
		if city.insideTheWall(point) {
			weight *= 2.5
		} else {
			weight *= 0.6
		}
	}
	neighbours, around := city.parcelCountAround(point, 1), city.parcelCountAround(point, 3)
	nearBuilt := 1.0
	if len(city.Parcels) > 0 {
		nearBuilt = math.Min(1, float64(around+1)/12)
	}
	weight = weight * nearBuilt / (1 + math.Pow(float64(neighbours)/12, 2))
	if city.Network.Degree(node) == 1 {
		weight *= city.organic().continuation()
	}
	city.nodeWeights[node] = weight
	return weight
}

// proposedDirection is where a new street from a node wants to go, and whether it is organic.
func (city *City) proposedDirection(node int) (float64, bool) {
	base := city.baseDirection(city.Network.Directions(node))
	point := city.Network.Nodes[node]
	axis, strength := city.fieldAxis(point)
	adjusted := closestAngle([]float64{axis, axis + math.Pi/2, axis + math.Pi, axis - math.Pi/2}, base)
	direction := base
	if math.Abs(angleDifference(adjusted, base)) < 40*math.Pi/180*(0.5+strength) && city.random.Float64() < city.organic().snapping() {
		direction = adjusted
	}
	_, planned := city.gridZoneAt(point)
	deviation := city.organic().wander()
	if planned {
		deviation = 3
	}
	return direction + city.random.NormFloat64()*deviation*math.Pi/180, !planned
}

// baseDirection: one street → straight on; two → sideways from the middle of a straight street, or
// outwards from a corner; three or more → into the widest gap.
func (city *City) baseDirection(directions []float64) float64 {
	switch len(directions) {
	case 1:
		return directions[0] + math.Pi
	case 2:
		if math.Abs(angleDifference(directions[0], directions[1])) > 140*math.Pi/180 {
			return directions[0] + math.Pi/2*[]float64{-1, 1}[city.random.IntN(2)]
		}
		mean := math.Atan2(math.Sin(directions[0])+math.Sin(directions[1]), math.Cos(directions[0])+math.Cos(directions[1]))
		return mean + math.Pi
	}
	return widestGap(directions)
}

func widestGap(directions []float64) float64 {
	angles := make([]float64, len(directions))
	for index, direction := range directions {
		angles[index] = math.Mod(math.Mod(direction, 2*math.Pi)+2*math.Pi, 2*math.Pi)
	}
	sort.Float64s(angles)
	best, bestGap := 0.0, -1.0
	for index, angle := range angles {
		gap := math.Mod(angles[(index+1)%len(angles)]-angle+2*math.Pi, 2*math.Pi)
		if gap > bestGap {
			best, bestGap = angle+gap/2, gap
		}
	}
	return best
}

func closestAngle(candidates []float64, target float64) float64 {
	best := candidates[0]
	for _, candidate := range candidates[1:] {
		if math.Abs(angleDifference(candidate, target)) < math.Abs(angleDifference(best, target)) {
			best = candidate
		}
	}
	return best
}

// fieldAxis is the main direction (modulo π) of the tensor field at a point and its strength (0..1):
// contour lines weighted by the slope, radial around the centres and the planned grid, added as
// tensors w·(cos 2θ, sin 2θ).
func (city *City) fieldAxis(point Point) (float64, float64) {
	type component struct{ angle, weight float64 }
	components := []component{{city.Terrain.ContourBearing(point.X, point.Z), math.Min(1, city.Terrain.Slope(point.X, point.Z)/0.08)}}
	for _, centre := range city.Scenario.CentresIn(city.Year) {
		if distance := point.DistanceTo(centre.point); distance > 1e-6 {
			components = append(components, component{centre.point.AngleTo(point), 0.35 * centre.weight * math.Exp(-math.Pow(distance/0.4, 2))})
		}
	}
	if zone, planned := city.gridZoneAt(point); planned {
		components = append(components, component{zone.AngleDegrees * math.Pi / 180, 1.2})
	}
	if pull, direction := city.riverPull(point); pull > 0.01 {
		components = append(components, component{direction, city.Scenario.River.alignment() * pull})
	}
	sumCosine, sumSine, sumWeight := 0.0, 0.0, 0.0
	for _, part := range components {
		sumCosine += part.weight * math.Cos(2*part.angle)
		sumSine += part.weight * math.Sin(2*part.angle)
		sumWeight += part.weight
	}
	if sumWeight == 0 {
		sumWeight = 1
	}
	return 0.5 * math.Atan2(sumSine, sumCosine), math.Hypot(sumCosine, sumSine) / sumWeight
}

// tryStreet proposes a street from a node and builds it if it passes the legality tests.
func (city *City) tryStreet(node int) bool {
	origin := city.Network.Nodes[node]
	end, direction, length := city.proposedEnd(node)
	if city.Terrain.Slope(end.X, end.Z) > steepestPlot {
		return false // nothing could be built along it
	}
	if !city.Window.Contains(pointGeometry(end)) {
		return false
	}
	end, followsContour := city.followContourIfSteep(origin, end, direction, length)
	if !followsContour {
		return false
	}
	piece := Segment{origin, end}
	if city.crossesObstacles(piece) {
		return false
	}
	bridge, allowed := city.riverCrossing(piece)
	if !allowed || city.crossesWallOutsideGates(piece) {
		return false
	}
	destination, found := city.streetDestination(node, piece)
	if !found {
		return false
	}
	return city.buildStreet(node, destination, bridge)
}

// proposedEnd is where a new street from a node wants to end: organic streets go some random length
// in the proposed direction; in a planned zone they run along the lattice, one block long, and end
// on a lattice node.
func (city *City) proposedEnd(node int) (Point, float64, float64) {
	origin := city.Network.Nodes[node]
	if zone, planned := city.gridZoneAt(origin); planned {
		base := city.baseDirection(city.Network.Directions(node))
		axis := zone.AngleDegrees * math.Pi / 180
		direction := closestAngle([]float64{axis, axis + math.Pi/2, axis + math.Pi, axis - math.Pi/2}, base)
		end := zone.snap(origin.Towards(direction, zone.blockSide()))
		return end, origin.AngleTo(end), origin.DistanceTo(end)
	}
	direction, _ := city.proposedDirection(node)
	segment := city.organic().segment()
	length := math.Max(0.02, segment+0.27*segment*city.random.NormFloat64())
	return origin.Towards(direction, length), direction, length
}

// followContourIfSteep: on a steep slope the street turns to the contour line, or gives up when
// that is more than 35° away from where it wanted to go.
func (city *City) followContourIfSteep(origin, end Point, direction, length float64) (Point, bool) {
	middle := Segment{origin, end}.Middle()
	if city.Terrain.Slope(middle.X, middle.Z) <= steepForStreets {
		return end, true
	}
	contour := city.Terrain.ContourBearing(origin.X, origin.Z)
	contour = closestAngle([]float64{contour, contour + math.Pi}, direction)
	if math.Abs(angleDifference(contour, direction)) > 35*math.Pi/180 {
		return end, false
	}
	return origin.Towards(contour, length), true
}

func (city *City) crossesObstacles(piece Segment) bool {
	return segmentGeometry(piece).Intersects(city.Obstacles)
}

// riverCrossing allows short bridges, now and then.
func (city *City) riverCrossing(piece Segment) (bridge, allowed bool) {
	line := segmentGeometry(piece)
	if !line.Intersects(city.Rivers) {
		return false, true
	}
	wet := line.Intersection(city.Rivers)
	if wet.Length() > city.longestBridge() || city.random.Float64() > city.bridgeChance() || !city.bridgeFits(piece, centroidOf(wet)) {
		return false, false
	}
	return true, true
}

func (city *City) crossesWallOutsideGates(piece Segment) bool {
	wall := city.CurrentWall()
	if wall == nil || !segmentGeometry(piece).Crosses(wall.Line) {
		return false
	}
	for _, gate := range wall.Gates {
		if piece.DistanceTo(gate) < gateReach {
			return false
		}
	}
	return true
}

// crossing is where a proposed street meets an existing one.
type crossing struct {
	distance float64
	edge     int
	point    Point
}

// streetDestination: the street ends at the first street it crosses; otherwise it snaps to a
// nearby node, or is lengthened ×1.5 to close against a street, or ends in a new node.
func (city *City) streetDestination(node int, piece Segment) (int, bool) {
	if crossings := city.crossingsOf(node, piece); len(crossings) > 0 {
		first := crossings[0]
		if first.distance < shortestStreet {
			return 0, false
		}
		return city.Network.SplitEdge(first.edge, first.point), true
	}
	lengthened := Segment{piece.From, piece.From.Add(piece.To.Subtract(piece.From).Scale(city.organic().closing()))}
	closings := city.crossingsOf(node, lengthened)
	skip := map[int]bool{node: true}
	for neighbour := range city.Network.neighbours[node] {
		skip[neighbour] = true
	}
	if magnet := city.Network.NearestNode(piece.To, city.organic().magnet(), skip); magnet >= 0 {
		return magnet, true
	}
	if len(closings) > 0 {
		return city.Network.SplitEdge(closings[0].edge, closings[0].point), true
	}
	return city.Network.AddNode(piece.To), true
}

// crossingsOf lists, nearest first, where a piece crosses streets not leaving from the node.
func (city *City) crossingsOf(node int, piece Segment) []crossing {
	var crossings []crossing
	for _, identifier := range city.Network.EdgesNearSegment(piece, 0.001) {
		edge := city.Network.Edges[identifier]
		if edge.From == node || edge.To == node {
			continue
		}
		if point, crosses := piece.Crossing(city.Network.Segment(edge)); crosses {
			crossings = append(crossings, crossing{piece.Project(point), identifier, point})
		}
	}
	sort.SliceStable(crossings, func(first, second int) bool { return crossings[first].distance < crossings[second].distance })
	return crossings
}

// buildStreet adds the street if the final piece is long enough, clear of obstacles and not too
// close in angle to the streets already leaving the node.
func (city *City) buildStreet(node, destination int, bridge bool) bool {
	origin, end := city.Network.Nodes[node], city.Network.Nodes[destination]
	final := Segment{origin, end}
	if final.Length() < shortestStreet || city.crossesObstacles(final) {
		return false
	}
	angle := origin.AngleTo(end)
	for _, existing := range city.Network.Directions(node) {
		if math.Abs(angleDifference(angle, existing)) < narrowestAngle {
			return false
		}
	}
	if city.Network.AddEdge(node, destination, ClassStreet, city.Year) == nil {
		return false
	}
	if bridge {
		city.Bridges = append(city.Bridges, final)
	}
	return true
}
