package urban

import (
	"math"
	"sort"
)

// How a house grows on its plot. By default (Conzen's burgage cycle) the building covers 0.4 of the
// plot and grows to 0.7 towards the back. With Scenario.BuildingDepth, houses are nearly square:
// they start as deep as the low end of the range times their front and grow to the high end;
// behind them the plot stays a garden. Either way, once a house cannot grow on the ground it
// gains floors, up to the usual maximum, or the rare maximum for the most wanted plots.

const deepeningShareOfFront = 0.25

func (city *City) squareHouses() bool { return city.Scenario.BuildingDepth != nil }

// startBuilding gives a new plot its first house.
func (city *City) startBuilding(parcel *Parcel) {
	if !city.squareHouses() || parcel.Use == UseExtensive {
		return
	}
	parcel.BuildingDepth = math.Min(parcel.Front*city.Scenario.BuildingDepth[0], parcel.Depth)
	parcel.Coverage = coverageOf(parcel)
}

// deepestBuilding is how deep the house of a plot can be.
func (city *City) deepestBuilding(parcel *Parcel) float64 {
	return math.Min(parcel.Front*city.houseDepthFactor(parcel.Centre), parcel.Depth)
}

// canDeepen tells whether the house can still grow towards the back.
func (city *City) canDeepen(parcel *Parcel) bool {
	if city.squareHouses() {
		// only a real step: the deepest house grows a little every year with the town's density, and
		// tiny steps would keep houses from ever gaining a floor
		return parcel.BuildingDepth < city.deepestBuilding(parcel)-0.1*parcel.Front
	}
	return parcel.Coverage < fullCoverage
}

// deepen grows the house one step towards the back.
func (city *City) deepen(parcel *Parcel) {
	if !city.squareHouses() {
		parcel.Coverage = math.Round(math.Min(fullCoverage, parcel.Coverage+coverageStep)*100) / 100
		return
	}
	parcel.BuildingDepth = math.Min(city.deepestBuilding(parcel), parcel.BuildingDepth+parcel.Front*deepeningShareOfFront)
	parcel.Coverage = coverageOf(parcel)
}

// coverageOf is the share of the plot a house of the plot's front and building depth covers.
func coverageOf(parcel *Parcel) float64 {
	if parcel.SquareMetres <= 0 {
		return 0
	}
	return math.Min(1, parcel.Front*(parcel.BuildingDepth+parcel.BackDepth)*1e6/parcel.SquareMetres)
}

// floorLimits gives the floors each plot may reach this year: the usual maximum, or the rare
// maximum for the most wanted share of the plots.
type floorLimits struct {
	usual, rare int
	taller      map[*Parcel]bool // plots that may reach the rare maximum
}

// floorLimitsNow works out this year's limits. The rare maximum is a quota, not a threshold: plots
// that already stand above the usual maximum keep their place in it, and only the most wanted of
// the rest fill what is left, so the share of tall houses stays at the scenario's share instead of
// growing as the most wanted places move with the town.
func (city *City) floorLimitsNow(pressures map[*Parcel]float64) floorLimits {
	limits := floorLimits{usual: city.Scenario.FloorsIn(city.Year)}
	rare := city.Scenario.RareFloors
	if rare == nil || rare.Floors <= limits.usual || rare.Share <= 0 {
		return limits
	}
	limits.rare, limits.taller = rare.Floors, map[*Parcel]bool{}
	quota := int(rare.Share * float64(len(pressures)))
	var others []*Parcel
	for parcel := range pressures {
		if parcel.Floors > limits.usual {
			limits.taller[parcel] = true
		} else {
			others = append(others, parcel)
		}
	}
	// a fixed order (the most wanted first, then the oldest) keeps runs repeatable
	sort.Slice(others, func(first, second int) bool {
		if pressures[others[first]] != pressures[others[second]] {
			return pressures[others[first]] > pressures[others[second]]
		}
		return others[first].Identifier < others[second].Identifier
	})
	for _, parcel := range others[:max(0, min(len(others), quota-len(limits.taller)))] {
		limits.taller[parcel] = true
	}
	return limits
}

// forParcel is the most floors a plot may reach this year, before the outskirts lower it.
func (limits floorLimits) forParcel(parcel *Parcel) int {
	if limits.taller[parcel] {
		return limits.rare
	}
	return limits.usual
}
