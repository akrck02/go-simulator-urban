package urban

import (
	"sort"
)

// Densification follows Conzen's burgage cycle (1960) and Caniggia and Maffei's typological
// process: before the city spreads, its plots fill up. Each year up to a quarter of the plots
// advance, the most wanted first: the building first takes the back of the plot (coverage up to
// 0.7) and then gains a floor.

const (
	firstCoverage  = 0.4
	fullCoverage   = 0.7
	coverageStep   = 0.1 // default growth (see buildings.go)
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
	city.layQuays()
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
	pressures := city.pressures()
	limits := city.floorLimitsNow(pressures)
	candidates := city.densifiableParcels(pressures, limits)
	sort.SliceStable(candidates, func(first, second int) bool { return pressures[candidates[first]] > pressures[candidates[second]] })
	gained := 0.0
	limit := max(1, int(advancingShare*float64(len(city.Parcels))))
	for _, parcel := range candidates[:min(limit, len(candidates))] {
		before := parcel.BuiltSquareMetres()
		city.advance(parcel, city.floorCapAt(parcel.Centre, limits.forParcel(parcel)))
		gained += parcel.BuiltSquareMetres() - before
		if gained >= wanted {
			break
		}
	}
	return gained
}

// pressures is how wanted every built plot is this year.
func (city *City) pressures() map[*Parcel]float64 {
	pressures := map[*Parcel]float64{}
	for _, parcel := range city.Parcels {
		if parcel.Use != UseExtensive && parcel.standing() {
			pressures[parcel] = city.pressureOn(parcel)
		}
	}
	return pressures
}

func (city *City) densifiableParcels(pressures map[*Parcel]float64, limits floorLimits) []*Parcel {
	var candidates []*Parcel
	for _, parcel := range city.Parcels {
		if _, built := pressures[parcel]; !built {
			continue
		}
		if city.canDeepen(parcel) || parcel.Floors < city.floorCapAt(parcel.Centre, limits.forParcel(parcel)) || city.canBuildAtTheBack(parcel) {
			candidates = append(candidates, parcel)
		}
	}
	return candidates
}

// advance moves a plot one step: the house grows towards the back, gains a floor or, in the dense
// core, the plot builds at its back. A plot that can do none of these stays as it is: it never
// goes past its floor limit.
func (city *City) advance(parcel *Parcel, cap int) {
	switch {
	case city.canDeepen(parcel):
		city.deepen(parcel)
	case parcel.Floors < cap:
		parcel.Floors++
	case city.canBuildAtTheBack(parcel):
		city.buildAtTheBack(parcel)
	}
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

// firstFloorsAt is the number of floors a new house gets at a place: one, or two when the year
// allows three or more, but never more than the outskirts allow there.
func (city *City) firstFloorsAt(point Point) int {
	usual := city.Scenario.FloorsIn(city.Year)
	if usual <= 2 {
		return 1
	}
	return min(2, city.floorCapAt(point, usual))
}
