package urban

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/akrck02/go-simulator/configuration"
	"github.com/akrck02/go-simulator/host"
	"github.com/akrck02/go-simulator/snapshot"
	"github.com/akrck02/go-simulator/world"
	"github.com/twpayne/go-geos"
)

// The plugin must work for any kind of town, not only the one it was ported for. These tests grow
// very different places on synthetic relief and check what must hold everywhere: nothing built in
// the water or on obstacles, no plots on top of each other, nothing outside the window, the floor
// space the people need, and a settled area close to what towns of that size covered.
//
// With URBAN_PLACES_OUTPUT=folder the final state of every place is saved there as GeoJSON (with
// its inputs), to look at with tools/view_snapshot.py. With -short only the small places run.

// place is a synthetic place: a relief, its inputs and the city rule's scenario.
type place struct {
	name     string
	relief   func(x, z float64) float64 // metres
	features []string                   // GeoJSON features
	scenario string                     // YAML lines of the city rule (indented 10 spaces)
	years    int
	large    bool
}

const placeHalfSide = 1.5 // km: every place is a 3 × 3 km square around the origin

func square(side float64) string {
	return polygonJSON([][2]float64{{-side, -side}, {side, -side}, {side, side}, {-side, side}})
}

func polygonJSON(points [][2]float64) string {
	ring := make([]string, 0, len(points)+1)
	for _, point := range append(points, points[0]) {
		ring = append(ring, fmt.Sprintf("[%g,%g]", point[0], point[1]))
	}
	return fmt.Sprintf(`{"type":"Polygon","coordinates":[[%s]]}`, strings.Join(ring, ","))
}

func lineJSON(points ...[2]float64) string {
	parts := make([]string, 0, len(points))
	for _, point := range points {
		parts = append(parts, fmt.Sprintf("[%g,%g]", point[0], point[1]))
	}
	return fmt.Sprintf(`{"type":"LineString","coordinates":[%s]}`, strings.Join(parts, ","))
}

func feature(role, geometry string) string {
	return fmt.Sprintf(`{"type":"Feature","properties":{"role":%q},"geometry":%s}`, role, geometry)
}

func riverFeature(width float64, geometry string) string {
	return fmt.Sprintf(`{"type":"Feature","properties":{"role":"river","width":%g},"geometry":%s}`, width, geometry)
}

func places() []place {
	return []place{
		{
			name: "coastal village", years: 60,
			// the land rises from the shore (z = 0.5) to the north; the sea is south of it
			relief: func(x, z float64) float64 { return math.Max(0, (0.5-z)*60) + 3*math.Sin(x*4) },
			features: []string{
				feature("window", square(1.4)),
				feature("obstacles", polygonJSON([][2]float64{{-1.6, 0.5}, {1.6, 0.5}, {1.6, 1.6}, {-1.6, 1.6}})),
				feature("road", lineJSON([2]float64{-1.6, 0.42}, [2]float64{1.6, 0.42})),
				feature("road", lineJSON([2]float64{0, 0.42}, [2]float64{0.1, -0.5}, [2]float64{0, -1.6})),
			},
			scenario: `          nucleus: [0, 0.38]
          population: {model: logistic, from_year: 0, from: 80, to_year: 60, to: 1200, ceiling: 1500}
          centres: [{at: [0, 0.38], weight: 1}]
          max_floors: [{floors: 2}]
          first_wall: {population: 1000000}`,
		},
		{
			name: "river market town", years: 50,
			relief: func(x, z float64) float64 { return 40 + 2*math.Abs(z) },
			features: []string{
				feature("window", square(1.4)),
				riverFeature(0.03, lineJSON([2]float64{-1.6, 0.0}, [2]float64{-0.3, -0.02}, [2]float64{0.3, 0.06}, [2]float64{1.6, 0.04})),
				feature("road", lineJSON([2]float64{-1.6, -0.5}, [2]float64{0, -0.3}, [2]float64{1.6, -0.6})),
				feature("road", lineJSON([2]float64{0.05, -1.6}, [2]float64{0, -0.3}, [2]float64{0, 0.4}, [2]float64{-0.1, 1.6})),
			},
			scenario: `          nucleus: [0, -0.3]
          population: {model: logistic, from_year: 0, from: 300, to_year: 50, to: 6000, ceiling: 7500}
          open_streets_below: 0.7
          centres: [{at: [0, -0.3], weight: 1}, {at: [0, 0.3], weight: 0.6, grows_from: 10, grows_for: 20}]
          max_floors: [{before: 25, floors: 2}, {floors: 3}]
          first_wall: {population: 3000}
          river: {reach: 0.25}`,
		},
		{
			name: "hilltop town", years: 50,
			// a hill 140 m high, the town around its top
			relief: func(x, z float64) float64 { return 140 * math.Exp(-(x*x+z*z)/(2*0.45*0.45)) },
			features: []string{
				feature("window", square(1.2)),
				feature("road", lineJSON([2]float64{-1.6, 0.2}, [2]float64{-0.8, 0.3}, [2]float64{-0.35, 0.1}, [2]float64{0, 0})),
				feature("road", lineJSON([2]float64{1.6, -0.4}, [2]float64{0.7, -0.5}, [2]float64{0.3, -0.25}, [2]float64{0, 0})),
			},
			scenario: `          nucleus: [0, 0]
          population: {model: logistic, from_year: 0, from: 200, to_year: 50, to: 2500, ceiling: 3200}
          open_streets_below: 0.7
          centres: [{at: [0, 0], weight: 1}]
          max_floors: [{floors: 2}]
          rare_floors: {floors: 3, share: 0.05}
          building_depth: [1, 1.5]
          first_wall: {population: 1200}`,
		},
		{
			name: "planned grid town", years: 40,
			relief: func(x, z float64) float64 { return 20 },
			features: []string{
				feature("window", square(1.4)),
				feature("road", lineJSON([2]float64{-1.6, 0}, [2]float64{1.6, 0})),
				feature("road", lineJSON([2]float64{0, -1.6}, [2]float64{0, 1.6})),
			},
			scenario: `          nucleus: [0, 0]
          population: {model: logistic, from_year: 0, from: 500, to_year: 40, to: 8000, ceiling: 10000}
          open_streets_below: 0.7
          centres: [{at: [0, 0], weight: 1}]
          grid: [{from_year: 0, angle_degrees: 0, area: [[-1.4,-1.4],[1.4,-1.4],[1.4,1.4],[-1.4,1.4]]}]
          max_floors: [{floors: 2}]
          first_wall: {year: 30, population: 6000}`,
		},
		{
			name: "valley street village", years: 60,
			// a narrow valley along x: steep sides 300 m from the axis
			relief: func(x, z float64) float64 { return 300 * math.Pow(math.Abs(z)/0.5, 2) },
			features: []string{
				feature("window", square(1.4)),
				feature("rivers", polygonJSON([][2]float64{{-1.6, 0.08}, {1.6, 0.06}, {1.6, 0.09}, {-1.6, 0.11}})),
				feature("road", lineJSON([2]float64{-1.6, -0.02}, [2]float64{0, 0}, [2]float64{1.6, -0.03})),
			},
			scenario: `          nucleus: [0, 0]
          population: {model: logistic, from_year: 0, from: 100, to_year: 60, to: 1500, ceiling: 2000}
          centres: [{at: [0, 0], weight: 1}]
          max_floors: [{floors: 2}]
          first_wall: {population: 1000000}`,
		},
		{
			name: "large plain city", years: 80, large: true,
			relief: func(x, z float64) float64 { return 30 + 5*math.Sin(x*2)*math.Cos(z*3) },
			features: []string{
				feature("window", square(1.45)),
				feature("road", lineJSON([2]float64{-1.6, 0.1}, [2]float64{1.6, -0.1})),
				feature("road", lineJSON([2]float64{0.1, -1.6}, [2]float64{-0.1, 1.6})),
				feature("road", lineJSON([2]float64{-1.6, -1.6}, [2]float64{1.6, 1.6})),
			},
			scenario: `          nucleus: [0, 0]
          population: {model: logistic, from_year: 0, from: 1000, to_year: 80, to: 80000, ceiling: 95000}
          open_streets_below: 0.5
          centres: [{at: [0, 0], weight: 1}, {at: [0.6, -0.5], weight: 0.5, grows_from: 30, grows_for: 30}]
          max_floors: [{before: 40, floors: 2}, {floors: 3}]
          first_wall: {population: 8000}`,
		},
	}
}

// writePlace writes the relief and the inputs of a place in a folder.
func writePlace(t *testing.T, directory string, current place) {
	t.Helper()
	const cell, cells = 0.0125, 257
	minimum := -1.6
	heights := make([]float32, 0, cells*cells)
	for row := range cells {
		for column := range cells {
			heights = append(heights, float32(current.relief(minimum+float64(column)*cell, minimum+float64(row)*cell)))
		}
	}
	writeNumpy(t, filepath.Join(directory, "relief.npy"), heights, cells, cells)
	collection := `{"type":"FeatureCollection","features":[` + strings.Join(current.features, ",") + `]}`
	if err := os.WriteFile(filepath.Join(directory, "inputs.geojson"), []byte(collection), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeNumpy(t *testing.T, path string, values []float32, rows, columns int) {
	t.Helper()
	header := fmt.Sprintf("{'descr': '<f4', 'fortran_order': False, 'shape': (%d, %d), }", rows, columns)
	for (10+len(header)+1)%64 != 0 {
		header += " "
	}
	header += "\n"
	contents := append([]byte("\x93NUMPY\x01\x00"), byte(len(header)), 0)
	contents = append(contents, header...)
	for _, value := range values {
		contents = binary.LittleEndian.AppendUint32(contents, math.Float32bits(value))
	}
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatal(err)
	}
}

const placeRules = `
plugins: [{name: urban, builtin: true}]
stages:
  - name: city
    clock: {unit: year, from: -1, to: %d, step: 1}
    rules:
      - rule: city
        name: place
        with:
          relief: {path: relief.npy, minimum_x: -1.6, minimum_z: -1.6, cell_size: 0.0125}
          features: inputs.geojson
          seed: 11
%s
      - {rule: densify, with: {city: place}}
      - {rule: expand, with: {city: place}}
      - {rule: walls, with: {city: place}}
      - {rule: prune_empty_streets, with: {city: place}, every: 5}
      - {rule: split_large_blocks, with: {city: place}, every: 5}
      - {rule: space_syntax, with: {city: place, samples: 150}, every: 5}
      - {rule: indicators, with: {city: place, years: [%d]}}
`

func TestDifferentKindsOfPlaces(t *testing.T) {
	output := os.Getenv("URBAN_PLACES_OUTPUT")
	for _, current := range places() {
		t.Run(current.name, func(t *testing.T) {
			if current.large && testing.Short() {
				t.Skip("large place: skipped with -short")
			}
			directory := t.TempDir()
			if output != "" {
				directory = filepath.Join(output, strings.ReplaceAll(current.name, " ", "-"))
				if err := os.MkdirAll(directory, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			writePlace(t, directory, current)
			engine, indicators := growPlace(t, directory, current)
			t.Logf("%s, year %d: %s", current.name, current.years, summary(indicators))
			checkPlace(t, engine.World, directory, indicators)
			if output != "" {
				if err := snapshot.WriteEntities(filepath.Join(directory, "final.geojson"), engine.World, map[string]any{"place": current.name}); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func growPlace(t *testing.T, directory string, current place) (*host.Engine, map[string]any) {
	t.Helper()
	scenario := current.scenario
	if threshold := os.Getenv("URBAN_OPEN_STREETS_BELOW"); threshold != "" && !strings.Contains(scenario, "open_streets_below") {
		scenario += "\n          open_streets_below: " + threshold
	}
	settings, err := configuration.Parse([]byte(fmt.Sprintf(placeRules, current.years, scenario, current.years)))
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("GO_SIMULATOR_RULES_DIRECTORY", directory)
	engine, err := host.Build(settings, nil, map[string]host.Builtin{"urban": Plugin().ServeConnection})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = engine.Close() })
	if _, err := engine.Start(); err != nil {
		t.Fatal(err)
	}
	var indicators map[string]any
	for !engine.Stage("city").Finished() {
		sets, err := engine.StepStage("city")
		if err != nil {
			t.Fatal(err)
		}
		for _, set := range sets {
			for _, change := range set.Changes {
				if change.Operation == world.Emit && change.Event.Name == "indicators" {
					indicators = change.Event.Data
				}
			}
		}
	}
	return engine, indicators
}

func summary(indicators map[string]any) string {
	keys := []string{"inhabitants", "settled_hectares", "expected_hectares_cesaretti", "inhabitants_per_hectare", "parcels",
		"street_pieces", "crossings_per_hectare", "mean_street_piece_metres", "mean_block_metres", "floor_area_ratio_far", "walls"}
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, fmt.Sprintf("%s %v", key, indicators[key]))
	}
	return strings.Join(parts, " · ")
}

// readInputs reads the polygons of a role from a place's inputs.
func readInputs(t *testing.T, directory, role string) *geos.Geom {
	t.Helper()
	contents, err := os.ReadFile(filepath.Join(directory, "inputs.geojson"))
	if err != nil {
		t.Fatal(err)
	}
	var collection struct {
		Features []struct {
			Properties map[string]any  `json:"properties"`
			Geometry   json.RawMessage `json:"geometry"`
		} `json:"features"`
	}
	if err := json.Unmarshal(contents, &collection); err != nil {
		t.Fatal(err)
	}
	result := emptyGeometry()
	for _, feature := range collection.Features {
		if feature.Properties["role"] == role {
			geometry, err := geos.NewGeomFromGeoJSON(string(feature.Geometry))
			if err != nil {
				t.Fatal(err)
			}
			result = unionOf(result, geometry)
		}
	}
	return result
}

func polygonOfShape(shape *world.Shape) *geos.Geom {
	points := make([]Point, 0, len(shape.Outline))
	for _, corner := range shape.Outline {
		points = append(points, Point{float64(corner[0]), float64(corner[1])})
	}
	return valid(polygonGeometry(points)) // float32 corners can leave a polygon slightly invalid
}

// checkPlace checks what must hold in any town.
func checkPlace(t *testing.T, current *world.World, directory string, indicators map[string]any) {
	t.Helper()
	window := readInputs(t, directory, "window")
	water := valid(unionOf(readInputs(t, directory, "rivers"), readInputs(t, directory, "obstacles"), riverWater(current)))
	var plots []*geos.Geom
	plotArea := 0.0
	for _, parcel := range current.EntitiesOfKind("parcel") {
		polygon := polygonOfShape(parcel.Shape)
		plots = append(plots, polygon)
		plotArea += polygon.Area()
		if !window.Contains(polygon.Centroid()) {
			t.Errorf("%s lies outside the window", parcel.Identifier)
		}
	}
	if len(plots) == 0 {
		t.Fatal("nothing was built")
	}
	built := valid(unionOf(plots...))
	wet := 0.0 // plot by plot: the union of thousands of float32 outlines is too fragile to cut at once
	for _, plot := range plots {
		if plot.Intersects(water) {
			if part := plot.Intersection(water); part != nil {
				wet += part.Area()
			}
		}
	}
	if wet > 0.001*built.Area() {
		t.Errorf("%.2f ha of plots stand in water or on obstacles", wet*100)
	}
	if overlap := plotArea - built.Area(); overlap > 0.001*plotArea {
		t.Errorf("plots overlap: %.3f ha counted twice", overlap*100)
	}
	for _, building := range current.EntitiesOfKind("building") {
		if water.Contains(polygonOfShape(building.Shape).Centroid()) {
			t.Errorf("%s stands in water or on an obstacle", building.Identifier)
		}
	}
	settled, expected := indicators["settled_hectares"].(float64), indicators["expected_hectares_cesaretti"].(float64)
	if settled < 0.5*expected || settled > 2.5*expected {
		t.Errorf("settled %v ha; towns of this size covered about %v ha", settled, expected)
	}
	needed := indicators["inhabitants"].(float64) * 30
	floorSpace := indicators["floor_area_ratio_far"].(float64) * settled * 1e4
	if floorSpace < 0.85*needed {
		t.Errorf("the people need %.0f m² of floor space; only about %.0f m² were built", needed, floorSpace)
	}
}

// riverWater is the water of the rivers the town publishes (axis widened to its width).
func riverWater(current *world.World) *geos.Geom {
	water := emptyGeometry()
	for _, river := range current.EntitiesOfKind("river") {
		points := make([]Point, 0, len(river.Shape.Line))
		for _, point := range river.Shape.Line {
			points = append(points, Point{float64(point[0]), float64(point[1])})
		}
		water = unionOf(water, lineGeometry(points).Buffer(float64(river.Shape.Width)/2*0.98, 8))
	}
	return water
}

func TestARiverShapesItsTown(t *testing.T) {
	var town place
	for _, candidate := range places() {
		if candidate.name == "river market town" {
			town = candidate
		}
	}
	directory := t.TempDir()
	writePlace(t, directory, town)
	engine, _ := growPlace(t, directory, town)
	quays, bridges := 0, len(engine.World.EntitiesOfKind("bridge"))
	for _, street := range engine.World.EntitiesOfKind("street") {
		if street.Properties["class"] == ClassQuay {
			quays++
		}
	}
	if quays < 10 || bridges < 2 {
		t.Fatalf("a town on a river should line it with quays and bridge it: %d quay pieces, %d bridges", quays, bridges)
	}
	if len(engine.World.EntitiesOfKind("river")) != 1 {
		t.Fatalf("the river should be published as water")
	}
	workshops := 0
	for _, building := range engine.World.EntitiesOfKind("building") {
		if building.Properties["use"] == UseWorkshop {
			workshops++
		}
	}
	if workshops == 0 {
		t.Fatalf("mills and workshops should gather by the water")
	}
}

// TestRunsAreRepeatable grows the same place twice: the same scenario must give the same town, so a
// version the author approves can be made again.
func TestRunsAreRepeatable(t *testing.T) {
	current := places()[0]
	var summaries []string
	for range 2 {
		directory := t.TempDir()
		writePlace(t, directory, current)
		_, indicators := growPlace(t, directory, current)
		summaries = append(summaries, summary(indicators))
	}
	if summaries[0] != summaries[1] {
		t.Errorf("two runs differ:\n%s\n%s", summaries[0], summaries[1])
	}
}
