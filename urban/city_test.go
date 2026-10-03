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
func writeFlatTown(t *testing.T) string {
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
	  {"type": "Feature", "properties": {"role": "road"}, "geometry": {"type": "LineString", "coordinates": [[0,-1.2],[0,1.2]]}}]}`
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
          max_floors: [{before: 20, floors: 2}, {floors: 3}]
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

func runTown(t *testing.T, years int) (*host.Engine, []world.ChangeSet) {
	t.Helper()
	directory := writeFlatTown(t)
	rules := fmt.Sprintf(townRules, years, filepath.Join(directory, "relief.npy"), filepath.Join(directory, "inputs.geojson"), years)
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
	engine, sets := runTown(t, 40)
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
