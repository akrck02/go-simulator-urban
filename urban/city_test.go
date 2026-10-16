package urban

import (
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-simulator/configuration"
	"github.com/akrck02/go-simulator/host"
	"github.com/akrck02/go-simulator/world"
)

// writeFlatTown writes a flat relief and the inputs of a town: a window 2 km wide and a road
// crossing it, with the village in the middle.
func writeFlatTown(t *testing.T, wallRole string) string {
	t.Helper()
	directory := t.TempDir()
	header := "{'descr': '<f4', 'fortran_order': False, 'shape': (101, 101), }"
	for (10+len(header)+1)%64 != 0 {
		header += " "
	}
	header += "\n"
	contents := append([]byte("\x93NUMPY\x01\x00"), byte(len(header)), 0)
	contents = append(contents, header...)
	cell := make([]byte, 4)
	binary.LittleEndian.PutUint32(cell, math.Float32bits(100))
	for range 101 * 101 {
		contents = append(contents, cell...)
	}
	if err := os.WriteFile(filepath.Join(directory, "relief.npy"), contents, 0o644); err != nil {
		t.Fatal(err)
	}
	features := `{"type": "FeatureCollection", "features": [
	  {"type": "Feature", "properties": {"role": "window"}, "geometry": {"type": "Polygon", "coordinates": [[[-1,-1],[1,-1],[1,1],[-1,1],[-1,-1]]]}},
	  {"type": "Feature", "properties": {"role": "road"}, "geometry": {"type": "LineString", "coordinates": [[-1.2,0],[1.2,0]]}},
	  {"type": "Feature", "properties": {"role": "road"}, "geometry": {"type": "LineString", "coordinates": [[0,-1.2],[0,1.2]]}},
	  {"type": "Feature", "properties": {"role": "%s"}, "geometry": {"type": "Polygon", "coordinates": [[[-0.6,-0.6],[0.6,-0.6],[0.6,0.6],[-0.6,0.6],[-0.6,-0.6]]]}}]}`
	features = fmt.Sprintf(features, wallRole)
	if err := os.WriteFile(filepath.Join(directory, "inputs.geojson"), []byte(features), 0o644); err != nil {
		t.Fatal(err)
	}
	return directory
}

const townRules = `
plugins: [{name: urban, builtin: true}]
stages:
  - name: city
    clock: {unit: year, from: -1, to: %d, step: 1}
    rules:
      - rule: city
        name: town
        with:
          relief: {path: %q, minimum_x: -1, minimum_z: -1, cell_size: 0.02}
          features: %q
          nucleus: [0, 0]
          population: {model: logistic, from_year: 0, from: 300, to_year: 40, to: 6000, ceiling: 8000}
          centres: [{at: [0, 0], weight: 1}]
%s
          first_wall: {year: 25, population: 2500}
          seed: 3
      - {rule: densify, with: {city: town}}
      - {rule: expand, with: {city: town}}
      - {rule: walls, with: {city: town}}
      - {rule: prune_empty_streets, with: {city: town}, every: 5}
      - {rule: split_large_blocks, with: {city: town}, every: 5}
      - {rule: space_syntax, with: {city: town, samples: 60}, every: 5}
      - {rule: indicators, with: {city: town, years: [%d]}}
`

func runTown(t *testing.T, years int, wallRole, extra string) (*host.Engine, []world.ChangeSet) {
	t.Helper()
	directory := writeFlatTown(t, wallRole)
	if extra == "" {
		extra = "          max_floors: [{before: 20, floors: 2}, {floors: 3}]"
	}
	rules := fmt.Sprintf(townRules, years, filepath.Join(directory, "relief.npy"), filepath.Join(directory, "inputs.geojson"), extra, years)
	settings, err := configuration.Parse([]byte(rules))
	if err != nil {
		t.Fatal(err)
	}
	engine, err := host.Build(settings, nil, map[string]host.Builtin{"urban": Plugin().ServeConnection})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	sets, err := engine.Start()
	if err != nil {
		t.Fatal(err)
	}
	for !engine.Stage("city").Finished() {
		stepped, err := engine.StepStage("city")
		if err != nil {
			t.Fatal(err)
		}
		sets = append(sets, stepped...)
	}
	return engine, sets
}

func TestATownGrowsHousesStreetsAndAWall(t *testing.T) {
	engine, sets := runTown(t, 40, "clear", "")
	count := func(kind string) int { return len(engine.World.EntitiesOfKind(kind)) }
	if count("parcel") < 200 || count("building") < 200 {
		t.Fatalf("6000 people need hundreds of plots: %d parcels, %d buildings", count("parcel"), count("building"))
	}
	if count("street") < 100 {
		t.Fatalf("the town should open streets: %d pieces", count("street"))
	}
	if count("wall") == 0 {
		t.Fatalf("a wall should go up by year 25")
	}
	gates := engine.World.EntitiesOfKind("gate")
	if len(gates) < 2 {
		t.Fatalf("both roads cross the wall, so it should have gates: %d", len(gates))
	}
	for _, gate := range gates {
		if gate.Transform == nil {
			t.Fatalf("gate %s should stand at a point", gate.Identifier)
		}
	}
	var indicators map[string]any
	for _, set := range sets {
		for _, change := range set.Changes {
			if change.Operation == world.Emit && change.Event.Name == "indicators" {
				indicators = change.Event.Data
			}
		}
	}
	if indicators == nil {
		t.Fatal("the indicators event is missing")
	}
	hectares, expected := indicators["settled_hectares"].(float64), indicators["expected_hectares_cesaretti"].(float64)
	t.Logf("year 40: %v", indicators)
	if hectares < 0.4*expected || hectares > 3*expected {
		t.Fatalf("settled %v ha, a medieval town of this size covered about %v ha", hectares, expected)
	}
}

func TestSquareHousesRareFloorsAndAGivenWall(t *testing.T) {
	extra := "          max_floors: [{floors: 2}]\n          rare_floors: {floors: 3, share: 0.05}\n          building_depth: [1, 1.5]"
	engine, _ := runTown(t, 40, "wall_outline", extra)
	buildings, threeFloors := engine.World.EntitiesOfKind("building"), 0
	for _, building := range buildings {
		floors := int(building.Properties["floors"].(float64))
		if floors > 3 {
			t.Fatalf("no building should pass 3 floors: %d", floors)
		}
		if floors == 3 {
			threeFloors++
		}
	}
	if share := float64(threeFloors) / float64(len(buildings)); share > 0.08 {
		t.Fatalf("3 floors should be rare: %.0f %% of %d buildings", share*100, len(buildings))
	}
	for _, parcel := range engine.World.EntitiesOfKind("parcel") {
		if parcel.Properties["use"] == UseExtensive {
			continue
		}
		front, depth := parcel.Properties["front"].(float64), parcel.Properties["building_depth"].(float64)
		if depth < front*0.999 || depth > front*1.501 {
			t.Fatalf("houses should be 1 to 1.5 times as deep as their front: front %.4f, depth %.4f", front, depth)
		}
	}
	walls := engine.World.EntitiesOfKind("wall")
	if len(walls) == 0 || len(engine.World.EntitiesOfKind("old_wall")) > 0 {
		t.Fatalf("one wall, with the given shape, and no older ones: %d walls", len(walls))
	}
	for _, point := range walls[0].Shape.Line {
		if math.Abs(math.Max(math.Abs(float64(point[0])), math.Abs(float64(point[1])))-0.6) > 1e-3 {
			t.Fatalf("the wall should follow the given square of side 1.2: point %v", point)
		}
	}
}

func TestWallHeightsFollowTheCalendar(t *testing.T) {
	heights := WallHeights{{60, 4}, {70, 4}, {100, 25}}
	for _, check := range []struct{ year, want float64 }{{40, 4}, {60, 4}, {70, 4}, {85, 14.5}, {100, 25}, {120, 25}} {
		if got := heights.At(check.year); math.Abs(got-check.want) > 1e-9 {
			t.Errorf("in year %v: %v m, want %v", check.year, got, check.want)
		}
	}
	if (WallHeights{{70, 4}, {60, 25}}).check() == nil {
		t.Error("years that go down must be refused")
	}
}

func TestWallRisesInPhases(t *testing.T) {
	rise := WallRise{From: 6, Height: 30, Years: 30}
	for _, check := range []struct{ years, want float64 }{{0, 6}, {15, 18}, {30, 30}, {50, 30}, {-2, 6}} {
		if got := rise.HeightAfter(check.years); math.Abs(got-check.want) > 1e-9 {
			t.Errorf("after %v years: %v m, want %v", check.years, got, check.want)
		}
	}
}
