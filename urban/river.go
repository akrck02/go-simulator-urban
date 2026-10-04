package urban

import (
	"math"

	"github.com/twpayne/go-geos"
)

// A river that runs through a town shapes it: streets run along and across its banks, quays line
// it once the town reaches it, bridges join both sides, plots by the water are wanted (water for
// trades, mills, transport) and workshops gather there. Rivers come as "river" features: a line
// along the axis with a "width" property (world units, km). Without them, or without the
// scenario's "river" settings, rivers are only obstacles crossed by short bridges.

const (
	defaultRiverReach = 0.25 // how far from the river its pull on streets and plots reaches
	quayStep          = 0.08 // quays are laid along the river in pieces this long
	quayWidth         = 0.007
	quayClearance     = 0.002
	waterfrontReach   = 0.06 // plots this close to the river can be workshops
)

// RiverSettings are the scenario's "river" settings.
type RiverSettings struct {
	Reach            float64 `json:"reach"`              // km; default 0.25
	Alignment        float64 `json:"alignment"`          // weight of the banks in the street directions; default 1
	FrontageBonus    float64 `json:"frontage_bonus"`     // extra pull on frontages by the water; default 1
	Quays            *bool   `json:"quays"`              // lay quays along the banks; default true
	LongestBridge    float64 `json:"longest_bridge"`     // km; default 1.6 × the river's width
	BridgeChance     float64 `json:"bridge_chance"`      // chance a street may bridge the river; default 0.6
	BridgeSpacing    float64 `json:"bridge_spacing"`     // km between bridges along the river; default 0.15
	WorkshopsByWater float64 `json:"workshops_by_water"` // share of new plots by the water that are workshops; default 0.3
}

// riverAxis is the axis of a river with its width.
type riverAxis struct {
	line  *geos.Geom
	width float64
	quays map[int]bool // pieces (index along the axis) that already have quays
}

func (settings *RiverSettings) reach() float64 { return positiveOr(settings.Reach, defaultRiverReach) }

func (settings *RiverSettings) alignment() float64 { return positiveOr(settings.Alignment, 1) }

func (settings *RiverSettings) frontageBonus() float64 { return positiveOr(settings.FrontageBonus, 1) }

func (settings *RiverSettings) laysQuays() bool { return settings.Quays == nil || *settings.Quays }

func (settings *RiverSettings) bridgeChance() float64 { return positiveOr(settings.BridgeChance, 0.6) }

func (settings *RiverSettings) bridgeSpacing() float64 {
	return positiveOr(settings.BridgeSpacing, 0.15)
}

// steepestBridgeAngle: a bridge crosses the river at least this square to it.
const steepestBridgeAngle = 60 * math.Pi / 180

// bridgeFits tells whether a new bridge may cross at a place: far enough from the other bridges and
// nearly square to the river. Without river settings any crossing fits.
func (city *City) bridgeFits(piece Segment, crossing Point) bool {
	if !city.riversShape() {
		return true
	}
	for _, bridge := range city.Bridges {
		if bridge.Middle().DistanceTo(crossing) < city.Scenario.River.bridgeSpacing() {
			return false
		}
	}
	_, direction := city.nearestRiver(crossing)
	turn := math.Abs(angleDifference(piece.From.AngleTo(piece.To), direction))
	return math.Min(turn, math.Pi-turn) >= steepestBridgeAngle
}

func (settings *RiverSettings) workshopsByWater() float64 {
	return positiveOr(settings.WorkshopsByWater, 0.3)
}

func positiveOr(value, fallback float64) float64 {
	if value > 0 {
		return value
	}
	return fallback
}

// addRiver records a river axis and makes its water an obstacle.
func (city *City) addRiver(line *geos.Geom, width float64) {
	for _, part := range linesOf(line) {
		city.RiverAxes = append(city.RiverAxes, &riverAxis{line: part, width: width, quays: map[int]bool{}})
		city.Rivers = unionOf(city.Rivers, part.Buffer(width/2, 8))
	}
}

// riversShape tells whether rivers shape the town (the scenario has river settings and there is a river).
func (city *City) riversShape() bool { return city.Scenario.River != nil && len(city.RiverAxes) > 0 }

// nearestRiver returns the distance to the nearest river axis and the direction of the river there.
func (city *City) nearestRiver(point Point) (float64, float64) {
	best, direction := math.Inf(1), 0.0
	geometry := pointGeometry(point)
	for _, axis := range city.RiverAxes {
		distance := axis.line.Distance(geometry)
		if distance < best {
			best, direction = distance, axisDirection(axis.line, geometry)
		}
	}
	return best, direction
}

// axisDirection is the direction of a line where it passes closest to a point.
func axisDirection(line, point *geos.Geom) float64 {
	along := line.Project(point)
	before, after := line.Interpolate(math.Max(0, along-0.01)), line.Interpolate(math.Min(line.Length(), along+0.01))
	return math.Atan2(after.Y()-before.Y(), after.X()-before.X())
}

// riverPull is how strongly the river pulls at a point (1 on the banks, fading with the reach).
func (city *City) riverPull(point Point) (float64, float64) {
	if !city.riversShape() {
		return 0, 0
	}
	distance, direction := city.nearestRiver(point)
	return math.Exp(-math.Pow(distance/city.Scenario.River.reach(), 2)), direction
}

// longestBridge is the longest crossing of water a street may bridge.
func (city *City) longestBridge() float64 {
	if !city.riversShape() {
		return longestBridge
	}
	if city.Scenario.River.LongestBridge > 0 {
		return city.Scenario.River.LongestBridge
	}
	widest := 0.0
	for _, axis := range city.RiverAxes {
		widest = math.Max(widest, axis.width)
	}
	return math.Max(longestBridge, 1.6*widest)
}

func (city *City) bridgeChance() float64 {
	if !city.riversShape() {
		return bridgeChance
	}
	return city.Scenario.River.bridgeChance()
}

// layQuays lines both banks with quays wherever the town has reached the river: pieces of the
// river within the growth radius of a centre, one piece at a time.
func (city *City) layQuays() {
	if !city.riversShape() || !city.Scenario.River.laysQuays() {
		return
	}
	radius := city.growthRadius()
	for _, axis := range city.RiverAxes {
		pieces := int(math.Ceil(axis.line.Length() / quayStep))
		for piece := range pieces {
			if axis.quays[piece] {
				continue
			}
			from, to := float64(piece)*quayStep, math.Min(axis.line.Length(), float64(piece+1)*quayStep)
			middle := axis.line.Interpolate((from + to) / 2)
			point := Point{middle.X(), middle.Y()}
			if city.closeness(point, radius, largest) < 0.35 || !city.Window.Contains(middle) {
				continue
			}
			axis.quays[piece] = true
			city.layQuayPiece(axis, from, to)
		}
	}
}

// layQuayPiece lays the quays of one piece of river, on both banks.
func (city *City) layQuayPiece(axis *riverAxis, from, to float64) {
	stretch := lineBetween(axis.line, from, to)
	offset := axis.width/2 + quayWidth/2 + quayClearance
	for _, side := range []float64{1, -1} {
		for _, bank := range linesOf(stretch.OffsetCurve(side*offset, 4, geos.BufJoinStyleRound, 5)) {
			points := coordinatesOf(bank)
			if len(points) >= 2 && city.Window.Contains(bank.Centroid()) && !bank.Intersects(city.Obstacles) {
				city.Network.AddPath(points, ClassQuay, city.Year, 0.04)
			}
		}
	}
}

// lineBetween returns the part of a line between two distances along it.
func lineBetween(line *geos.Geom, from, to float64) *geos.Geom {
	points := []Point{}
	first, last := line.Interpolate(from), line.Interpolate(to)
	points = append(points, Point{first.X(), first.Y()})
	for _, vertex := range coordinatesOf(line) {
		position := line.Project(pointGeometry(vertex))
		if position > from && position < to {
			points = append(points, vertex)
		}
	}
	points = append(points, Point{last.X(), last.Y()})
	return lineGeometry(points)
}

// waterfrontUse makes some plots by the water workshops (mills, tanneries, dyers).
func (city *City) waterfrontUse(parcel *Parcel) {
	if !city.riversShape() || parcel.Use != UseHousing {
		return
	}
	if distance, _ := city.nearestRiver(parcel.Centre); distance < waterfrontReach && city.random.Float64() < city.Scenario.River.workshopsByWater() {
		parcel.Use = UseWorkshop
	}
}
