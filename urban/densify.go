package urban

import (
	"math"
	"sort"
)

// Densification follows Conzen's burgage cycle (1960) and Caniggia and Maffei's typological
// process: before the city spreads, its plots fill up. Each year up to a quarter of the plots
// advance, the most wanted first: the building first takes the back of the plot (coverage up to
// 0.7) and then gains a floor.

const (
	firstCoverage  = 0.4
	fullCoverage   = 0.7
	coverageStep   = 0.1
	advancingShare = 0.25
)

// startYear sets the year, the demand and what is missing; the first year it opens a square
// where the village was.
func (city *City) startYear(year float64) {
	city.Year = year
	if len(city.Squares) == 0 && city.Scenario.Nucleus != nil {
		nucleus := Point{city.Scenario.Nucleus[0], city.Scenario.Nucleus[1]}
		city.Squares = append(city.Squares, pointGeometry(nucleus).Buffer(0.018, 8))
	}
	demand := city.Population() * city.Scenario.SquareMetresPerPerson
	city.Deficit = demand - city.BuiltSquareMetres()
}

// densify fills existing plots. When the city covers less ground than cities of its size did, it
// only covers part of the deficit this way and spreads for the rest.
func (city *City) densify() {
	if city.Deficit <= 0 {
		return
	}
	share := 1.0
	if city.settledHectares() < 0.9*expectedHectares(city.Population()) {
		share = 0.3
	}
	city.Deficit -= city.densifyBy(city.Deficit * share)
}

func (city *City) densifyBy(wanted float64) float64 {
	if len(city.Parcels) == 0 {
		return 0
	}
	maximumFloors := city.Scenario.FloorsIn(city.Year)
	candidates := city.densifiableParcels(maximumFloors)
	pressures := make(map[*Parcel]float64, len(candidates))
	for _, parcel := range candidates {
		pressures[parcel] = city.pressureOn(parcel)
	}
	sort.SliceStable(candidates, func(first, second int) bool { return pressures[candidates[first]] > pressures[candidates[second]] })
	gained := 0.0
	limit := max(1, int(advancingShare*float64(len(city.Parcels))))
	for _, parcel := range candidates[:min(limit, len(candidates))] {
		before := parcel.BuiltSquareMetres()
		advance(parcel)
		gained += parcel.BuiltSquareMetres() - before
		if gained >= wanted {
			break
		}
	}
	return gained
}

func (city *City) densifiableParcels(maximumFloors int) []*Parcel {
	var candidates []*Parcel
	for _, parcel := range city.Parcels {
		if parcel.Use == UseExtensive || !parcel.standing() {
			continue
		}
		if parcel.Coverage < fullCoverage || parcel.Floors < maximumFloors {
			candidates = append(candidates, parcel)
		}
	}
	return candidates
}

// advance moves a plot one step through the burgage cycle.
func advance(parcel *Parcel) {
	if parcel.Coverage < fullCoverage {
		parcel.Coverage = math.Round(math.Min(fullCoverage, parcel.Coverage+coverageStep)*100) / 100
		return
	}
	parcel.Floors++
}

// pressureOn is how wanted a plot is: near the centres, inside the wall, on a shopping street.
func (city *City) pressureOn(parcel *Parcel) float64 {
	pressure := city.closeness(parcel.Centre, 0.5, largest)
	if city.insideTheWall(parcel.Centre) {
		pressure += 0.5
	}
	if parcel.Use == UseShop {
		pressure += 0.5
	}
	return pressure
}

// firstFloors is the number of floors new houses get.
func (city *City) firstFloors() int {
	if city.Scenario.FloorsIn(city.Year) <= 2 {
		return 1
	}
	return 2
}
