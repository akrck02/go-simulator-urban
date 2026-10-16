package urban

import (
	"testing"

	"github.com/akrck02/go-simulator/world"
)

// plotsWithPressures makes plots, the first the most wanted, and their pressures.
func plotsWithPressures(count int) ([]*Parcel, map[*Parcel]float64) {
	parcels := make([]*Parcel, 0, count)
	pressures := map[*Parcel]float64{}
	for index := range count {
		parcel := &Parcel{Identifier: index, Floors: 1, Use: UseHousing}
		parcels = append(parcels, parcel)
		pressures[parcel] = float64(count - index)
	}
	return parcels, pressures
}

func cityWithRareFloors(share float64) *City {
	return &City{Scenario: &Scenario{MaximumFloors: []FloorLimit{{Floors: 2}}, RareFloors: &RareFloors{Floors: 3, Share: share}}}
}

func TestOnlyTheMostWantedShareMayGoHigher(t *testing.T) {
	parcels, pressures := plotsWithPressures(100)
	limits := cityWithRareFloors(0.05).floorLimitsNow(pressures)
	for index, parcel := range parcels {
		want := 2
		if index < 5 {
			want = 3
		}
		if got := limits.forParcel(parcel); got != want {
			t.Fatalf("plot %d of 100 (the most wanted first) may reach %d floors, want %d", index, got, want)
		}
	}
}

func TestTallHousesKeepTheirPlaceInTheQuota(t *testing.T) {
	parcels, pressures := plotsWithPressures(100)
	// the town moved on: five plots that were the most wanted once are now among the least wanted
	for _, parcel := range parcels[90:95] {
		parcel.Floors = 3
	}
	limits := cityWithRareFloors(0.05).floorLimitsNow(pressures)
	for index, parcel := range parcels {
		tall := limits.forParcel(parcel) == 3
		if tall != (index >= 90 && index < 95) {
			t.Fatalf("plot %d: may go higher = %v; the quota of 5 is full with the plots already standing at 3 floors", index, tall)
		}
	}
}

func TestNoRareFloorsWithoutAShare(t *testing.T) {
	parcels, pressures := plotsWithPressures(10)
	limits := cityWithRareFloors(0).floorLimitsNow(pressures)
	if got := limits.forParcel(parcels[0]); got != 2 {
		t.Fatalf("without a share, even the most wanted plot stays at the usual 2 floors: %d", got)
	}
}

func TestAPlotAtItsLimitDoesNotGainAFloor(t *testing.T) {
	city := &City{Scenario: &Scenario{}}
	parcel := &Parcel{Floors: 2, Coverage: fullCoverage, Use: UseHousing}
	city.advance(parcel, 2)
	if parcel.Floors != 2 {
		t.Fatalf("a full plot at its limit of 2 floors grew to %d", parcel.Floors)
	}
}

func TestNewHousesInTheOutskirtsKeepToTheirFloors(t *testing.T) {
	city := &City{Year: 50, Scenario: &Scenario{MaximumFloors: []FloorLimit{{Floors: 3}}, Density: &DensitySettings{OuterFloors: 1},
		Centres: []GrowthCentre{{At: [2]float64{0, 0}, Weight: 1}}, Population: PopulationCurve{From: 300, To: 6000, ToYear: 100, Ceiling: 8000}}}
	if got := city.firstFloorsAt(Point{0, 0}); got != 2 {
		t.Errorf("a new house in the core of a town of 3 floors starts with 2 floors, not %d", got)
	}
	if got := city.firstFloorsAt(Point{20, 20}); got != 1 {
		t.Errorf("a new house far in the outskirts, where 1 floor is the most, starts with %d floors", got)
	}
}

func TestFloorsStayWithinTheirLimitsAllAlong(t *testing.T) {
	extra := "          max_floors: [{floors: 2}]\n          rare_floors: {floors: 3, share: 0.03}\n" +
		"          density: {outer_floors: 1, back_buildings_from: 0.6}"
	_, sets := runTown(t, 40, "clear", extra)
	floors := map[string]int{}
	for _, set := range sets {
		for _, change := range set.Changes {
			if change.Operation == world.Remove {
				delete(floors, string(change.Identifier))
				continue
			}
			if change.Entity == nil || change.Entity.Kind != "parcel" {
				continue
			}
			if use := change.Entity.Properties["use"]; use == UseExtensive || use == UseDemolished {
				delete(floors, string(change.Entity.Identifier))
				continue
			}
			floors[string(change.Entity.Identifier)] = int(change.Entity.Properties["floors"].(float64))
		}
		tall := 0
		for identifier, count := range floors {
			if count > 3 {
				t.Fatalf("year %v: %s has %d floors; the most is 3", set.Time, identifier, count)
			}
			if count == 3 {
				tall++
			}
		}
		if allowed := int(0.03*float64(len(floors))) + 1; tall > allowed {
			t.Fatalf("year %v: %d of %d plots have 3 floors; the share of 3 %% allows about %d", set.Time, tall, len(floors), allowed)
		}
	}
}
