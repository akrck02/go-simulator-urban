package urban

import (
	"math"

	"github.com/twpayne/go-geos"
)

// No published model simulates city walls, so this is the plugin's own rule: the first wall goes
// up at a population or, at the latest, in a year; it follows the concave hull of what is built,
// tied to the supporting rock when there is one, with gates where the roads cross it. When the
// suburbs outside hold more than a share of the floor space and the wall is at least 12 years
// old, a new one goes up and the old one becomes a ring road with a belt of gardens and convents
// (Whitehand 1967).

const (
	wallMinimumAge   = 12
	gateSpacing      = 0.06
	beltWidth        = 0.05
	ringRoadStep     = 0.05
	shortestRingRoad = 0.05
)

// wallsIfDue raises a wall when it is time.
func (city *City) wallsIfDue() {
	wall := city.CurrentWall()
	if wall == nil {
		if city.Population() >= city.Scenario.FirstWall.Population || city.Year >= city.Scenario.FirstWallYear() {
			city.raiseWall()
		}
		return
	}
	outside, total := city.floorSpaceOutside(wall), city.BuiltSquareMetres()
	if total <= 0 {
		total = 1
	}
	if outside/total > city.Scenario.NewWallWhenOutside && city.Year-wall.Year >= wallMinimumAge {
		city.raiseWall()
	}
}

func (city *City) floorSpaceOutside(wall *Wall) float64 {
	outside := 0.0
	for _, parcel := range city.Parcels {
		if parcel.Use == UseDemolished || parcel.Use == UseExtensive {
			continue
		}
		if !wall.Polygon.Contains(pointGeometry(parcel.Centre)) {
			outside += parcel.BuiltSquareMetres()
		}
	}
	return outside
}

// settledFootprint is everything built (plots and squares) grown by a margin.
func (city *City) settledFootprint(margin float64) *geos.Geom {
	var pieces []*geos.Geom
	for _, parcel := range city.Parcels {
		if parcel.standing() {
			pieces = append(pieces, parcel.Polygon)
		}
	}
	pieces = append(pieces, city.Squares...)
	if len(pieces) == 0 {
		return emptyGeometry()
	}
	return unionOf(pieces...).Buffer(margin, 8)
}

func (city *City) raiseWall() {
	settled := city.settledFootprint(0.03)
	if settled.IsEmpty() {
		return
	}
	outline := city.wallOutline(settled)
	if outline == nil {
		return
	}
	line := outline.ExteriorRing().Difference(city.WallSupport.Buffer(wallClearance, 8))
	gates := city.gatesOn(line)
	if old := city.CurrentWall(); old != nil {
		city.turnIntoRing(old)
	}
	city.Walls = append(city.Walls, &Wall{Polygon: outline, Line: line, Gates: gates, Year: city.Year})
	city.removeParcelsIn(line.Buffer(wallClearance, 8))
}

// wallOutline: the concave hull of what is built and of the old walls, smoothed, tied to the rock
// when it is close and kept inside the window.
func (city *City) wallOutline(settled *geos.Geom) *geos.Geom {
	parts := []*geos.Geom{settled}
	for _, wall := range city.Walls {
		parts = append(parts, wall.Polygon)
	}
	outline := unionOf(parts...).ConcaveHull(0.35, 0)
	outline = outline.Buffer(0.06, 8).Buffer(-0.02, 8).Simplify(0.01)
	if !city.WallSupport.IsEmpty() && outline.Distance(city.WallSupport) < 0.15 {
		outline = unionOf(outline, city.WallSupport.Buffer(0.01, 8)).Buffer(0.12, 8).Buffer(-0.12, 8)
	}
	return largestPolygon(valid(outline.Intersection(city.Window)))
}

// gatesOn puts a gate where a road or a main street crosses the wall line, at least gateSpacing apart.
func (city *City) gatesOn(line *geos.Geom) []Point {
	var gates []Point
	for _, identifier := range city.Network.SortedEdgeIdentifiers() {
		edge := city.Network.Edges[identifier]
		if edge.Class != ClassRoad && edge.Class != ClassMain {
			continue
		}
		for _, point := range pointsOf(segmentGeometry(city.Network.Segment(edge)).Intersection(line)) {
			if farFromAll(point, gates, gateSpacing) {
				gates = append(gates, point)
			}
		}
	}
	return gates
}

func farFromAll(point Point, others []Point, distance float64) bool {
	for _, other := range others {
		if point.DistanceTo(other) <= distance {
			return false
		}
	}
	return true
}

// turnIntoRing makes an old wall a ring road with a belt around it.
func (city *City) turnIntoRing(old *Wall) {
	city.Belts = unionOf(city.Belts, old.Line.Buffer(beltWidth, 8))
	for _, stretch := range linesOf(old.Line) {
		if stretch.Length() > shortestRingRoad {
			city.Network.AddPath(coordinatesOf(stretch), ClassRing, city.Year, ringRoadStep)
		}
	}
}

// ---------------------------------------------------------------- planned streets nobody builds on

// pruneEmptyStreets removes, from the outside in, dead ends without plots laid out more than some
// years ago (Weber et al. 2009: a planned street is only built if needed).
func (city *City) pruneEmptyStreets(graceYears float64) {
	withParcels := map[int]bool{}
	for _, parcel := range city.Parcels {
		if parcel.standing() {
			withParcels[parcel.Edge] = true
		}
	}
	for removed := true; removed; {
		removed = false
		for _, identifier := range city.Network.SortedEdgeIdentifiers() {
			edge := city.Network.Edges[identifier]
			if city.isEmptyDeadEnd(edge, withParcels, graceYears) {
				city.Network.RemoveEdge(identifier)
				removed = true
			}
		}
	}
}

func (city *City) isEmptyDeadEnd(edge *Edge, withParcels map[int]bool, graceYears float64) bool {
	if (edge.Class != ClassStreet && edge.Class != ClassAlley) || withParcels[edge.Identifier] {
		return false
	}
	if city.Year-edge.Year < graceYears {
		return false
	}
	return min(city.Network.Degree(edge.From), city.Network.Degree(edge.To)) == 1
}

// ---------------------------------------------------------------- blocks too big: an alley (Siksna 1997)

// Blocks returns the blocks the streets enclose.
func (city *City) Blocks() []*geos.Geom {
	lines := make([]*geos.Geom, 0, len(city.Network.Edges))
	for _, identifier := range city.Network.SortedEdgeIdentifiers() {
		lines = append(lines, segmentGeometry(city.Network.Segment(city.Network.Edges[identifier])))
	}
	if len(lines) == 0 {
		return nil
	}
	noded := unionOf(lines...)
	var blocks []*geos.Geom
	for _, block := range polygonsOf(geos.Polygonize([]*geos.Geom{noded})) {
		if block.Area() > 1e-5 {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

// splitLargeBlocks cuts an alley across the short side of blocks wider than a size.
func (city *City) splitLargeBlocks(maximumSide float64) {
	for _, block := range city.Blocks() {
		city.splitBlock(block, maximumSide)
	}
}

func (city *City) splitBlock(block *geos.Geom, maximumSide float64) {
	rectangle := block.MinimumRotatedRectangle()
	if rectangle.TypeID() != geos.TypeIDPolygon {
		return
	}
	corners := coordinatesOf(rectangle.ExteriorRing())
	if len(corners) < 4 {
		return
	}
	shortSide := shortestSide(corners[:4])
	if shortSide.Length() < maximumSide || !city.Window.Contains(block.Centroid()) {
		return
	}
	centre := centroidOf(block)
	across := shortSide.Scale(1 / shortSide.Length())
	cutLine := segmentGeometry(Segment{centre.Subtract(across.Scale(shortSide.Length())), centre.Add(across.Scale(shortSide.Length()))})
	cut := cutLine.Intersection(block)
	if cut.TypeID() != geos.TypeIDLineString || cut.Length() < 0.05 || cut.Intersects(city.Obstacles) {
		return
	}
	city.layAlley(coordinatesOf(cut))
}

func shortestSide(corners []Point) Point {
	shortest := corners[1].Subtract(corners[0])
	for index := range corners {
		side := corners[(index+1)%len(corners)].Subtract(corners[index])
		if side.Length() < shortest.Length() {
			shortest = side
		}
	}
	return shortest
}

// layAlley joins the cut's ends to the streets they touch and clears the plots in its way.
func (city *City) layAlley(cut []Point) {
	var ends []int
	for _, point := range []Point{cut[0], cut[len(cut)-1]} {
		end, found := city.nodeOnStreetAt(point)
		if !found {
			return
		}
		ends = append(ends, end)
	}
	if city.Network.AddEdge(ends[0], ends[1], ClassAlley, city.Year) == nil {
		return
	}
	alley := Segment{city.Network.Nodes[ends[0]], city.Network.Nodes[ends[1]]}
	city.removeParcelsIn(segmentGeometry(alley).Buffer(widthOf(ClassAlley)/2+0.001, 8))
}

// nodeOnStreetAt finds the street within 3 m of a point and returns a node there (an existing one
// within 6 m, or a new one splitting the street).
func (city *City) nodeOnStreetAt(point Point) (int, bool) {
	best, bestDistance := -1, math.Inf(1)
	for _, identifier := range city.Network.EdgesNear(point, point, 0.002) {
		if distance := city.Network.Segment(city.Network.Edges[identifier]).DistanceTo(point); distance < bestDistance {
			best, bestDistance = identifier, distance
		}
	}
	if best < 0 || bestDistance > 0.003 {
		return 0, false
	}
	onStreet := city.Network.Segment(city.Network.Edges[best]).Closest(point)
	if near := city.Network.NearestNode(onStreet, 0.006, nil); near >= 0 {
		return near, true
	}
	return city.Network.SplitEdge(best, onStreet), true
}
