package urban

import (
	"math"
	"sort"

	"github.com/twpayne/go-geos"
)

// New plots are laid as series of burgage plots along the most wanted free street frontages
// (Conzen 1960; Slater 1987): one to two perches of front (a perch is 5.03 m) and 30 to 55 m deep,
// square to the street; in the belts where old walls stood, large extensive plots.

const (
	perch             = 0.00503 // km
	frontageMargin    = 0.008   // free ground left at each end of a street piece, at crossings and dead ends
	bendMargin        = 0.0015  // free ground left where a street only bends
	shortestFrontage  = 0.015
	frontagesPerRound = 12
	wantedFrontage    = 0.3 // frontages less wanted than this wait: a street nearby opens first
	steepestPlot      = 0.4
	smallestPlotShare = 0.55
	surroundingsReach = 0.07
	wallClearance     = 0.012
)

// bestFreeFrontage is the priority of the most wanted free frontage (0 when there is none).
func (city *City) bestFreeFrontage() float64 {
	city.refreshYearlyWeights()
	best := 0.0
	for _, side := range city.freeFrontages() {
		if _, known := city.frontagePriority[side.edge]; !known {
			city.frontagePriority[side.edge] = city.frontagePriorityOf(side.edge)
		}
		best = math.Max(best, city.frontagePriority[side.edge])
	}
	return best
}

// buildFrontages lays plots on the most wanted free frontages until the floor space wanted is
// covered; it returns the floor space gained.
func (city *City) buildFrontages(wanted float64) float64 {
	free := city.freeFrontages()
	if len(free) == 0 {
		return 0
	}
	city.refreshYearlyWeights()
	for _, side := range free {
		if _, known := city.frontagePriority[side.edge]; !known {
			city.frontagePriority[side.edge] = city.frontagePriorityOf(side.edge)
		}
	}
	free = city.wantedFrontages(free)
	gained := 0.0
	for _, side := range free[:min(frontagesPerRound, len(free))] {
		city.frontagesDone[side] = true
		for _, parcel := range city.parcelsAlong(side) {
			city.storeParcel(parcel)
			gained += parcel.BuiltSquareMetres()
		}
		if gained >= wanted {
			break
		}
	}
	return gained
}

func (city *City) freeFrontages() []frontage {
	var free []frontage
	for _, identifier := range city.Network.SortedEdgeIdentifiers() {
		edge := city.Network.Edges[identifier]
		if city.Network.Segment(edge).Length() <= shortestFrontage {
			continue
		}
		for _, side := range []int{-1, 1} {
			if candidate := (frontage{identifier, side}); !city.frontagesDone[candidate] {
				free = append(free, candidate)
			}
		}
	}
	return free
}

func (city *City) wantedFrontages(free []frontage) []frontage {
	var wanted []frontage
	for _, side := range free {
		if city.frontagePriority[side.edge] > wantedFrontage {
			wanted = append(wanted, side)
		}
	}
	sort.SliceStable(wanted, func(first, second int) bool {
		return city.frontagePriority[wanted[first].edge] > city.frontagePriority[wanted[second].edge]
	})
	return wanted
}

// refreshYearlyWeights forgets last year's frontage priorities and node weights: the city changes
// little within a year, so they are computed once a year.
func (city *City) refreshYearlyWeights() {
	if city.weightsYear == city.Year {
		return
	}
	city.weightsYear = city.Year
	city.frontagePriority, city.nodeWeights = map[int]float64{}, map[int]float64{}
}

func (city *City) frontagePriorityOf(identifier int) float64 {
	edge := city.Network.Edges[identifier]
	middle := city.Network.Segment(edge).Middle()
	priority := city.closeness(middle, city.growthRadius(), largest)
	if city.insideTheWall(middle) {
		priority *= 1.5
	}
	if edge.Class == ClassRoad || edge.Class == ClassMain || edge.Class == ClassQuay {
		priority *= 1.3
	}
	if pull, _ := city.riverPull(middle); pull > 0 {
		priority *= 1 + city.Scenario.River.frontageBonus()*pull
	}
	return priority + 0.02*city.random.Float64()
}

// parcelsAlong cuts plots along one side of a street piece, avoiding streets, obstacles, rivers,
// other plots, squares and walls.
func (city *City) parcelsAlong(side frontage) []*Parcel {
	edge := city.Network.Edges[side.edge]
	segment := city.Network.Segment(edge)
	length := segment.Length()
	direction := segment.To.Subtract(segment.From).Scale(1 / length)
	normal := Point{-direction.Z, direction.X}.Scale(float64(side.side))
	inBelt := city.Belts.Contains(pointGeometry(segment.Middle()))
	forbidden := city.forbiddenAround(segment)
	var parcels []*Parcel
	startMargin, endMargin := city.marginAt(edge.From), city.marginAt(edge.To)
	for position := startMargin; position < length-endMargin; {
		front, depth, use, coverage := city.plotSize(inBelt, segment.From.Add(direction.Scale(position)))
		front = math.Min(front, length-endMargin-position)
		if front < perch*0.8 {
			break
		}
		origin := segment.From.Add(direction.Scale(position)).Add(normal.Scale(widthOf(edge.Class) / 2))
		position += front
		plot := polygonGeometry([]Point{origin, origin.Add(direction.Scale(front)),
			origin.Add(direction.Scale(front)).Add(normal.Scale(depth)), origin.Add(normal.Scale(depth))})
		piece := city.fitPlot(plot, forbidden)
		if piece == nil {
			continue
		}
		parcel := &Parcel{Polygon: piece, Centre: centroidOf(piece), Edge: side.edge, Front: front, Depth: depth,
			Direction: direction, Normal: normal, SquareMetres: piece.Area() * 1e6, Coverage: coverage,
			Floors: city.firstFloors(), Use: use, Year: city.Year}
		city.startBuilding(parcel)
		city.waterfrontUse(parcel)
		parcels = append(parcels, parcel)
		forbidden.add(piece)
	}
	return parcels
}

// marginAt is the free ground a frontage leaves at a node: room for the corner at a crossing or a
// dead end, little where the street only bends and the row of houses goes on (plots that would
// overlap on the inner side of the bend are cut by fitPlot).
func (city *City) marginAt(node int) float64 {
	if city.Network.Degree(node) == 2 {
		return bendMargin
	}
	return frontageMargin
}

// plotSize draws the size of the next plot.
func (city *City) plotSize(inBelt bool, place Point) (front, depth float64, use string, coverage float64) {
	if inBelt {
		return uniform(city.random, 0.03, 0.06), uniform(city.random, 0.05, 0.08), UseExtensive, 0.25
	}
	if city.Scenario.Density != nil {
		front, depth = city.plotSizeAt(place)
		return front, depth, UseHousing, firstCoverage
	}
	perches := []float64{1.0, 1.5, 1.5, 2.0}[city.random.IntN(4)]
	return perch * perches, uniform(city.random, 0.03, 0.055), UseHousing, firstCoverage
}

// fitPlot cuts what is forbidden out of a plot and keeps the biggest piece, if it is big enough,
// inside the window and not too steep.
func (city *City) fitPlot(plot *geos.Geom, forbidden *forbiddenGround) *geos.Geom {
	rest := forbidden.cut(plot, smallestPlotShare*plot.Area())
	if rest == nil || rest.IsEmpty() {
		return nil
	}
	piece := largestPolygon(rest)
	if piece == nil || piece.Area() < smallestPlotShare*plot.Area() {
		return nil
	}
	centre := centroidOf(piece)
	if !city.Window.Contains(pointGeometry(centre)) || city.Terrain.Slope(centre.X, centre.Z) > steepestPlot {
		return nil
	}
	return piece
}

// forbiddenGround is the ground where no plot may go, kept as separate pieces: a plot is small, so
// subtracting only the pieces whose box touches it is much cheaper than joining them all.
type forbiddenGround struct {
	pieces  []*geos.Geom
	lowest  []Point
	highest []Point
}

func (ground *forbiddenGround) add(geometry *geos.Geom) {
	if geometry == nil || geometry.IsEmpty() {
		return
	}
	low, high := boundsOf(geometry)
	ground.pieces, ground.lowest, ground.highest = append(ground.pieces, geometry), append(ground.lowest, low), append(ground.highest, high)
}

// cut subtracts the forbidden pieces from a plot; it gives up (nil) as soon as less than some area is left.
func (ground *forbiddenGround) cut(plot *geos.Geom, smallestArea float64) *geos.Geom {
	low, high := boundsOf(plot)
	rest := plot
	for index, piece := range ground.pieces {
		if ground.highest[index].X < low.X || ground.lowest[index].X > high.X || ground.highest[index].Z < low.Z || ground.lowest[index].Z > high.Z {
			continue
		}
		if !piece.Intersects(rest) {
			continue
		}
		rest, _ = safeDifference(rest, piece)
		if rest.IsEmpty() || rest.Area() < smallestArea {
			return nil
		}
	}
	return rest
}

// plotReach is how far from its street a plot can reach (deepest plot plus margin): obstacles and
// rivers further away cannot touch it, so they are clipped off before the union.
const plotReach = 0.12

// forbiddenAround is the ground near a street piece where no plot may go: obstacles, rivers,
// streets, other plots, squares and walls.
func (city *City) forbiddenAround(segment Segment) *forbiddenGround {
	low, high := boxOf(segment)
	clip := func(geometry *geos.Geom) *geos.Geom {
		return geometry.ClipByRect(low.X-plotReach, low.Z-plotReach, high.X+plotReach, high.Z+plotReach)
	}
	ground := &forbiddenGround{}
	for _, piece := range polygonsOf(clip(city.Obstacles)) {
		ground.add(piece)
	}
	for _, piece := range polygonsOf(clip(city.Rivers)) {
		ground.add(piece)
	}
	for _, identifier := range city.Network.EdgesNearSegment(segment, surroundingsReach) {
		edge := city.Network.Edges[identifier]
		ground.add(segmentGeometry(city.Network.Segment(edge)).Buffer(widthOf(edge.Class)/2, 8))
	}
	reach := Point{surroundingsReach, surroundingsReach}
	for _, parcel := range city.parcelsNear(low.Subtract(reach), high.Add(reach)) {
		ground.add(parcel)
	}
	for _, square := range city.Squares {
		ground.add(square)
	}
	for _, wall := range city.Walls {
		ground.add(clip(wall.clearance()))
	}
	return ground
}

func uniform(random interface{ Float64() float64 }, lowest, highest float64) float64 {
	return lowest + (highest-lowest)*random.Float64()
}
