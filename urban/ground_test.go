package urban

import (
	"encoding/json"
	"fmt"
	"math"
	"path/filepath"
	"slices"
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

func TestRiversCanComeFromTheWorld(t *testing.T) {
	const years = 8
	directory := writeFlatTown(t, "clear")
	rules := fmt.Sprintf(townRules, years, filepath.Join(directory, "relief.npy"), filepath.Join(directory, "inputs.geojson"),
		"          max_floors: [{floors: 2}]\n          river: {from_world: true, narrowest: 0.01}", years)
	settings, err := configuration.Parse([]byte(rules))
	if err != nil {
		t.Fatal(err)
	}
	ground := world.New()
	river := func(identifier string, z, width float32) world.Change {
		return world.AddEntity(&world.Entity{Identifier: world.Identifier(identifier), Kind: world.KindRiver,
			Shape: &world.Shape{Line: [][2]float32{{-1.5, z}, {0, z + 0.05}, {1.5, z}}, Width: width}, Properties: map[string]any{"width": float64(width)}})
	}
	for _, change := range []world.Change{river("wide", 0.3, 0.02), river("stream", -0.4, 0.003), river("far away", 5, 0.02)} {
		if err := ground.ApplyChange(change); err != nil {
			t.Fatal(err)
		}
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
	var taken []*world.Entity
	for _, entity := range engine.World.EntitiesOfKind(world.KindRiver) {
		if strings.HasPrefix(string(entity.Identifier), "town-river-") {
			taken = append(taken, entity)
		}
	}
	if len(taken) != 1 {
		t.Fatalf("the town should take only the wide river near it, not the stream nor the far one: %d rivers", len(taken))
	}
	for _, parcel := range engine.World.EntitiesOfKind("parcel") {
		for _, point := range parcel.Shape.Outline {
			if math.Abs(float64(point[1])-0.3-0.05*(1-math.Abs(float64(point[0]))/1.5)) < 0.009 && math.Abs(float64(point[0])) < 1 {
				t.Fatalf("parcel %s is built in the river taken from the world: %v", parcel.Identifier, point)
			}
		}
	}
}

func TestTheTownFollowsTheGroundOfTheWorld(t *testing.T) {
	directory := writeFlatTown(t, "clear")
	field := elevationOf(t, filepath.Join(directory, "relief.npy"))
	current := world.New()
	if err := current.ApplyChange(world.SetFieldValues(field)); err != nil {
		t.Fatal(err)
	}
	city := &City{Name: "town", Scenario: &Scenario{}, groundValues: slices.Clone(field.Values)}
	city.Terrain, _ = TerrainFromField(current.Field(world.FieldElevation))
	before := city.Terrain
	if err := city.followGround(current); err != nil || city.Terrain != before {
		t.Fatalf("the same ground should not be taken again (%v)", err)
	}
	lowered := slices.Clone(field.Values)
	lowered[0] -= 5 // the erosion rule cut a little into the ground
	if err := current.ApplyChange(world.SetFieldValues(&world.FieldPatch{Name: world.FieldElevation, Columns: field.Columns, Rows: field.Rows,
		Extent: field.Extent, Values: lowered})); err != nil {
		t.Fatal(err)
	}
	if err := city.followGround(current); err != nil {
		t.Fatal(err)
	}
	if city.Terrain == before || city.Terrain.heights[0] != float64(lowered[0]) {
		t.Fatal("the town should take the ground again when the world's elevation changes")
	}
}
