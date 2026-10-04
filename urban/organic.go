package urban

import "math"

// How organic a town grows, and how its density falls from the centre. Organic streets are laid
// in short pieces that wander a little at every step and only sometimes bend to the tensor field,
// so they curve (Strano et al. 2012; Emilien et al. 2012). Density falls from the centre outwards
// (Clark 1951; Alexander et al. 1977, "density rings"): narrow, shallow plots with deep and taller
// houses in the core, wider and deeper plots with small houses and gardens in the outskirts. Areas
// of the scenario ("districts") can set their own density. Without these settings the town grows as
// before.

// OrganicSettings are the scenario's "organic" settings.
type OrganicSettings struct {
	Segment       float64  `json:"segment"`        // mean length of an organic street piece (km); default 0.055
	WanderDegrees float64  `json:"wander_degrees"` // spread of the turn at every piece; default 12
	FieldSnapping *float64 `json:"field_snapping"` // chance a street bends to the tensor field when close to it; default 1
	Continuation  float64  `json:"continuation"`   // how much more likely a street goes on from its end than branches; default 1
	Closing       float64  `json:"closing"`        // how far ahead (× the piece) a street looks to close a block; default 1.5
	Infill        float64  `json:"infill"`         // extra want of frontages among built plots, so streets fill before new ones open; default 0
}

func (settings *OrganicSettings) segment() float64 { return positiveOr(settings.Segment, 0.055) }

func (settings *OrganicSettings) wander() float64 { return positiveOr(settings.WanderDegrees, 12) }

func (settings *OrganicSettings) continuation() float64 { return positiveOr(settings.Continuation, 1) }

func (settings *OrganicSettings) closing() float64 {
	return positiveOr(settings.Closing, closingFactor)
}

// magnet is how close a node must be for a new street to snap to it.
func (settings *OrganicSettings) magnet() float64 {
	return magnetRadius * settings.closing() / closingFactor
}

func (settings *OrganicSettings) snapping() float64 {
	if settings.FieldSnapping == nil {
		return 1
	}
	return *settings.FieldSnapping
}

// organic returns the organic settings, or the defaults (the former behaviour).
func (city *City) organic() *OrganicSettings {
	if city.Scenario.Organic != nil {
		return city.Scenario.Organic
	}
	return &OrganicSettings{}
}

// DensitySettings are the scenario's "density" settings: plots and houses from the core to the outskirts.
type DensitySettings struct {
	CoreFrontPerches  [2]float64 `json:"core_front_perches"`  // default [1, 1.5]
	OuterFrontPerches [2]float64 `json:"outer_front_perches"` // default [3, 5]
	CoreDepth         [2]float64 `json:"core_depth"`          // km; default [0.025, 0.04]
	OuterDepth        [2]float64 `json:"outer_depth"`         // km; default [0.035, 0.05]
	CoreHouseDepth    float64    `json:"core_house_depth"`    // deepest house, × the front; default 2
	OuterHouseDepth   float64    `json:"outer_house_depth"`   // default 1
	OuterFloors       int        `json:"outer_floors"`        // most floors in the outskirts; default 1
	RadiusFactor      float64    `json:"radius_factor"`       // the core's radius, × the growth radius; default 0.6
	BackBuildingsFrom float64    `json:"back_buildings_from"` // density from which full plots build at the back, round a yard; default 0 (never)
	Districts         []District `json:"districts"`
}

// District is an area with its own density (0 outskirts … 1 core), whatever its distance to the centre.
type District struct {
	Name    string       `json:"name"`
	Area    [][2]float64 `json:"area"`
	Density float64      `json:"density"`
}

func pairOr(value, fallback [2]float64) [2]float64 {
	if value == [2]float64{} {
		return fallback
	}
	return value
}

// densityAt is how central a place is, from 0 (outskirts) to 1 (core); −1 without density settings.
func (city *City) densityAt(point Point) float64 {
	settings := city.Scenario.Density
	if settings == nil {
		return -1
	}
	geometry := pointGeometry(point)
	for _, district := range settings.Districts {
		if len(district.Area) >= 3 && polygonGeometry(pointsFrom(district.Area)).Contains(geometry) {
			return clamp(district.Density, 0, 1)
		}
	}
	radius := city.growthRadius() * positiveOr(settings.RadiusFactor, 0.6)
	return clamp(city.closeness(point, radius, largest), 0, 1)
}

func pointsFrom(pairs [][2]float64) []Point {
	points := make([]Point, 0, len(pairs))
	for _, pair := range pairs {
		points = append(points, Point{pair[0], pair[1]})
	}
	return points
}

func lerp(from, to, share float64) float64 { return from + (to-from)*share }

// plotSizeAt draws the size of a plot for its place: from the core's narrow and shallow plots to
// the outskirts' wide and deep ones.
func (city *City) plotSizeAt(point Point) (front, depth float64) {
	settings, density := city.Scenario.Density, city.densityAt(point)
	core, outer := pairOr(settings.CoreFrontPerches, [2]float64{1, 1.5}), pairOr(settings.OuterFrontPerches, [2]float64{3, 5})
	coreDepth, outerDepth := pairOr(settings.CoreDepth, [2]float64{0.025, 0.04}), pairOr(settings.OuterDepth, [2]float64{0.035, 0.05})
	perches := uniform(city.random, lerp(outer[0], core[0], density), lerp(outer[1], core[1], density))
	return perch * perches, uniform(city.random, lerp(outerDepth[0], coreDepth[0], density), lerp(outerDepth[1], coreDepth[1], density))
}

// houseDepthFactor is how deep a house may grow, × its front, at a place.
func (city *City) houseDepthFactor(point Point) float64 {
	settings := city.Scenario.Density
	if settings == nil {
		return city.Scenario.BuildingDepth[1]
	}
	return lerp(positiveOr(settings.OuterHouseDepth, 1), positiveOr(settings.CoreHouseDepth, 2), city.densityAt(point))
}

// infillBonus raises the want of a frontage among built plots (a street fills before new ones open).
func (city *City) infillBonus(middle Point) float64 {
	infill := city.organic().Infill
	if infill <= 0 {
		return 1
	}
	return 1 + infill*math.Min(1, float64(city.parcelCountAround(middle, 1))/infillPlots)
}

const (
	infillPlots      = 20     // plots around a frontage from which it counts as fully among built plots
	smallestYard     = 0.004  // km between the house and the building at the back
	backShareOfFront = 0.8    // the building at the back is at most this deep, × the front
	shallowestBack   = 0.0035 // km; shallower buildings at the back are not worth it
)

// canBuildAtTheBack tells whether a full plot of the dense core can still build at its back.
func (city *City) canBuildAtTheBack(parcel *Parcel) bool {
	settings := city.Scenario.Density
	if settings == nil || settings.BackBuildingsFrom <= 0 || parcel.BackDepth > 0 || parcel.Use == UseExtensive {
		return false
	}
	return city.densityAt(parcel.Centre) >= settings.BackBuildingsFrom && backRoom(parcel) >= shallowestBack
}

// backRoom is how deep the building at the back of a plot can be.
func backRoom(parcel *Parcel) float64 {
	return math.Min(parcel.Front*backShareOfFront, parcel.Depth-parcel.BuildingDepth-smallestYard)
}

// buildAtTheBack raises the building at the back of a plot.
func (city *City) buildAtTheBack(parcel *Parcel) {
	parcel.BackDepth = backRoom(parcel)
	parcel.Coverage = coverageOf(parcel)
}

// floorCapAt lowers the floor limit in the outskirts.
func (city *City) floorCapAt(point Point, usual int) int {
	settings := city.Scenario.Density
	if settings == nil {
		return usual
	}
	outer := max(1, settings.OuterFloors)
	return int(math.Round(lerp(float64(min(outer, usual)), float64(usual), city.densityAt(point))))
}
