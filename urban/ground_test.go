package urban

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/go-simulator/configuration"
	"github.com/akrck02/go-simulator/host"
	"github.com/akrck02/go-simulator/world"
)

func TestColoursComeFromTheScenario(t *testing.T) {
	extra := "          max_floors: [{floors: 2}]\n          colours: {street: \"#ff0000\"}"
	engine, _ := runTown(t, 6, "clear", extra)
	street, building := engine.World.EntitiesOfKind("street")[0], engine.World.EntitiesOfKind("building")[0]
	if *street.Shape.Color != linear("#ff0000") {
		t.Errorf("street colour %v, want the scenario's red", *street.Shape.Color)
	}
	if *building.Shape.Color != linear(fallbackColour) {
		t.Errorf("building colour %v, want the fallback grey when the scenario gives none", *building.Shape.Color)
	}
}

func TestColoursMustBeHex(t *testing.T) {
	_, err := scenarioFrom(map[string]any{"population": map[string]any{"model": "logistic", "from_year": 0.0, "from": 1.0,
		"to_year": 1.0, "to": 2.0, "ceiling": 3.0}, "centres": []any{map[string]any{"at": []any{0.0, 0.0}, "weight": 1.0}},
		"colours": map[string]any{"street": "red"}})
	if err == nil || !strings.Contains(err.Error(), "#rrggbb") {
		t.Errorf("a colour that is not #rrggbb must fail, got %v", err)
	}
}

// TestTheGroundCanComeFromTheWorld grows the same town twice: once on its relief file and once on
// the same relief published as the world's elevation field. Both towns must be the same.
func TestTheGroundCanComeFromTheWorld(t *testing.T) {
	const years = 8
	fromFile, _ := runTown(t, years, "clear", "")

	directory := writeFlatTown(t, "clear")
	rules := fmt.Sprintf(townRules, years, filepath.Join(directory, "relief.npy"), filepath.Join(directory, "inputs.geojson"),
		"          max_floors: [{before: 20, floors: 2}, {floors: 3}]", years)
	rules = strings.Replace(rules, fmt.Sprintf("          relief: {path: %q, minimum_x: -1, minimum_z: -1, cell_size: 0.02}\n",
		filepath.Join(directory, "relief.npy")), "", 1)
	settings, err := configuration.Parse([]byte(rules))
	if err != nil {
		t.Fatal(err)
	}
	ground := world.New()
	if err := ground.ApplyChange(world.SetFieldValues(elevationOf(t, filepath.Join(directory, "relief.npy")))); err != nil {
		t.Fatal(err)
	}
	engine, err := host.Build(settings, ground, map[string]host.Builtin{"urban": Plugin().ServeConnection})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	for !engine.Stage("city").Finished() {
		if _, err := engine.StepStage("city"); err != nil {
			t.Fatal(err)
		}
	}
	if first, second := describe(t, fromFile.World), describe(t, engine.World); first != second {
		t.Errorf("the town on the elevation field differs from the town on the relief file (%d and %d bytes)", len(first), len(second))
	}
}

// elevationOf is the relief file of writeFlatTown as an elevation field: cell centres from −1, 0.02 apart.
func elevationOf(t *testing.T, path string) *world.FieldPatch {
	t.Helper()
	terrain, err := LoadTerrain(TerrainFile{Path: path, MinimumX: -1, MinimumZ: -1, CellSize: 0.02})
	if err != nil {
		t.Fatal(err)
	}
	values := make([]float32, len(terrain.heights))
	for index, height := range terrain.heights {
		values[index] = float32(height)
	}
	half := terrain.CellSize / 2
	extent := &world.Extent{MinimumX: -1 - half, MaximumX: -1 - half + float64(terrain.Columns)*terrain.CellSize,
		MinimumZ: -1 - half, MaximumZ: -1 - half + float64(terrain.Rows)*terrain.CellSize}
	return &world.FieldPatch{Name: world.FieldElevation, Columns: terrain.Columns, Rows: terrain.Rows, Extent: extent, Values: values}
}

func describe(t *testing.T, current *world.World) string {
	t.Helper()
	encoded, err := json.Marshal(current.Entities())
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}
