package urban

import (
	"math"
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
	return math.Min(1, parcel.Front*parcel.BuildingDepth*1e6/parcel.SquareMetres)
}

// floorLimits gives the floors each plot may reach this year: the usual maximum, or the rare
// maximum for the most wanted share of the plots.
type floorLimits struct {
	usual, rare int
	threshold   float64 // pressure from which a plot is among the most wanted
}

func (city *City) floorLimitsNow(pressures map[*Parcel]float64) floorLimits {
	limits := floorLimits{usual: city.Scenario.FloorsIn(city.Year), threshold: math.Inf(1)}
	rare := city.Scenario.RareFloors
	if rare == nil || rare.Floors <= limits.usual || rare.Share <= 0 {
		return limits
	}
	values := make([]float64, 0, len(pressures))
	for _, pressure := range pressures {
		values = append(values, pressure)
	}
	limits.rare, limits.threshold = rare.Floors, quantile(values, 1-rare.Share)
	return limits
}

func (limits floorLimits) forPressure(pressure float64) int {
	if limits.rare > 0 && pressure >= limits.threshold {
		return limits.rare
	}
	return limits.usual
}
