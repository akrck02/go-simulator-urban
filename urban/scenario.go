package urban

import (
	"encoding/json"
	"fmt"
	"math"
)

// Scenario is everything the simulator does not invent: where the city grows, how many people
// live in it each year and what was there before. It comes from the parameters of the city rule:
//
//	relief:      {path: relief.npy, minimum_x: -1, minimum_z: -1, cell_size: 0.0125}
//	features:    inputs.geojson     # window, roads, rivers, obstacles, wall_support (property "role")
//	nucleus:     [0, 0]             # the crossroads of the first village
//	population:  {model: logistic, from_year: 0, from: 300, to_year: 100, to: 6000, ceiling: 8000}
//	centres:     [{at: [0, 0], weight: 1}, {at: [0.3, 0.8], weight: 0.9, grows_from: 30, grows_for: 40}]
//	grid:        [{from_year: 40, angle_degrees: 20, area: [[x, z], ...], block: 0.08}]
//	max_floors:  [{before: 40, floors: 2}, {before: 80, floors: 3}, {floors: 4}]
//	first_wall:  {year: 60, population: 3000}
//	new_wall_when_outside: 0.3
//	square_metres_per_person: 30
//	seed: 7
//	open_streets_below: 0.5                 # 0.3: street villages; higher: compact towns with back streets
//	rare_floors: {floors: 3, share: 0.05}   # only the most wanted 5 % of plots reach 3 floors
//	building_depth: [1, 1.5]                # houses nearly square: 1 to 1.5 times as deep as their front
//	clear_nodes: [buildings, grounds, paths]   # what the drawn scene had where the city grows
//	tree_nodes:  [broadleaf forest, laurel forest]
type Scenario struct {
	Relief                TerrainFile     `json:"relief"`
	Features              string          `json:"features"`
	Nucleus               *[2]float64     `json:"nucleus"`
	Population            PopulationCurve `json:"population"`
	Centres               []GrowthCentre  `json:"centres"`
	Grid                  []GridZone      `json:"grid"`
	MaximumFloors         []FloorLimit    `json:"max_floors"`
	FirstWall             FirstWall       `json:"first_wall"`
	NewWallWhenOutside    float64         `json:"new_wall_when_outside"`
	SquareMetresPerPerson float64         `json:"square_metres_per_person"`
	Seed                  uint64          `json:"seed"`
	HeightScale           float64         `json:"height_scale"`       // how much buildings are exaggerated in 3D (1 = true height)
	OpenStreetsBelow      float64         `json:"open_streets_below"` // a town opens streets before building on frontages less wanted than this
	River                 *RiverSettings  `json:"river"`              // rivers shape the town (see river.go)
	RareFloors            *RareFloors     `json:"rare_floors"`        // a few of the most wanted plots may go higher
	BuildingDepth         *[2]float64     `json:"building_depth"`     // houses as deep as [from, to] times their front; nil = coverage 0.4 → 0.7
	ClearNodes            []string        `json:"clear_nodes"`        // scene nodes cleared under the "clear" features when the city starts
	TreeNodes             []string        `json:"tree_nodes"`         // scene nodes of trees, cleared there too and under every new plot
}

// PopulationCurve gives the number of inhabitants each year.
type PopulationCurve struct {
	Model    string  `json:"model"` // "logistic"
	FromYear float64 `json:"from_year"`
	From     float64 `json:"from"`
	ToYear   float64 `json:"to_year"`
	To       float64 `json:"to"`
	Ceiling  float64 `json:"ceiling"`
}

// GrowthCentre pulls the city towards a point; its weight can grow over some years.
type GrowthCentre struct {
	At        [2]float64 `json:"at"`
	Weight    float64    `json:"weight"`
	GrowsFrom *float64   `json:"grows_from"`
	GrowsFor  float64    `json:"grows_for"`
}

// GridZone is an area laid out on a grid from a year on.
type GridZone struct {
	FromYear     float64      `json:"from_year"`
	AngleDegrees float64      `json:"angle_degrees"`
	Area         [][2]float64 `json:"area"`
	Block        float64      `json:"block"` // side of the blocks (km); streets run along the lattice (default 0.08)
}

// blockSide is the side of the zone's blocks.
func (zone GridZone) blockSide() float64 {
	if zone.Block > 0 {
		return zone.Block
	}
	return 0.08
}

// snap moves a point to the nearest lattice node of the zone (the lattice goes through the origin).
func (zone GridZone) snap(point Point) Point {
	angle := zone.AngleDegrees * math.Pi / 180
	cosine, sine, side := math.Cos(angle), math.Sin(angle), zone.blockSide()
	along, across := point.X*cosine+point.Z*sine, -point.X*sine+point.Z*cosine
	along, across = math.Round(along/side)*side, math.Round(across/side)*side
	return Point{along*cosine - across*sine, along*sine + across*cosine}
}

// FloorLimit is the maximum number of floors before a year (the last one, without "before", holds after).
type FloorLimit struct {
	Before *float64 `json:"before"`
	Floors int      `json:"floors"`
}

// RareFloors lets the most wanted share of plots reach more floors than the usual maximum.
type RareFloors struct {
	Floors int     `json:"floors"`
	Share  float64 `json:"share"`
}

// FirstWall says when the first wall goes up: at a population or, at the latest, in a year.
type FirstWall struct {
	Year       *float64 `json:"year"`
	Population float64  `json:"population"`
}

// scenarioFrom reads the scenario from the city rule's parameters and fills in the defaults.
func scenarioFrom(parameters map[string]any) (*Scenario, error) {
	encoded, err := json.Marshal(parameters)
	if err != nil {
		return nil, err
	}
	scenario := &Scenario{}
	if err := json.Unmarshal(encoded, scenario); err != nil {
		return nil, fmt.Errorf("city parameters: %w", err)
	}
	scenario.fillDefaults()
	return scenario, scenario.check()
}

func (scenario *Scenario) fillDefaults() {
	if scenario.NewWallWhenOutside <= 0 {
		scenario.NewWallWhenOutside = 0.3
	}
	if scenario.OpenStreetsBelow <= 0 {
		scenario.OpenStreetsBelow = wantedFrontage
	}
	if scenario.SquareMetresPerPerson <= 0 {
		scenario.SquareMetresPerPerson = 30
	}
	if scenario.FirstWall.Population <= 0 {
		scenario.FirstWall.Population = 3000
	}
	if scenario.Seed == 0 {
		scenario.Seed = 1
	}
	if scenario.HeightScale <= 0 {
		scenario.HeightScale = 1
	}
	if len(scenario.MaximumFloors) == 0 {
		scenario.MaximumFloors = []FloorLimit{{Floors: 3}}
	}
}

func (scenario *Scenario) check() error {
	if scenario.Relief.Path == "" {
		return fmt.Errorf("city: relief is missing")
	}
	if scenario.Population.Model != "logistic" {
		return fmt.Errorf("city: population model %q is unknown (known: logistic)", scenario.Population.Model)
	}
	if scenario.Population.ToYear == scenario.Population.FromYear || scenario.Population.Ceiling <= scenario.Population.To {
		return fmt.Errorf("city: the population curve needs two different years and a ceiling above the final population")
	}
	if len(scenario.Centres) == 0 {
		return fmt.Errorf("city: at least one growth centre is needed")
	}
	return nil
}

// PopulationIn returns the inhabitants in a year: a logistic through both points.
func (scenario *Scenario) PopulationIn(year float64) float64 {
	curve := scenario.Population
	ceiling := curve.Ceiling
	rate := -math.Log((ceiling/curve.To-1)/(ceiling/curve.From-1)) / (curve.ToYear - curve.FromYear)
	return ceiling / (1 + (ceiling/curve.From-1)*math.Exp(-rate*(year-curve.FromYear)))
}

// weightedPoint is a growth centre at a given year.
type weightedPoint struct {
	point  Point
	weight float64
}

// CentresIn returns the growth centres and their weights in a year.
func (scenario *Scenario) CentresIn(year float64) []weightedPoint {
	centres := make([]weightedPoint, 0, len(scenario.Centres))
	for _, centre := range scenario.Centres {
		weight := centre.Weight
		if centre.GrowsFrom != nil && centre.GrowsFor > 0 {
			weight *= clamp((year-*centre.GrowsFrom)/centre.GrowsFor, 0, 1)
		}
		centres = append(centres, weightedPoint{Point{centre.At[0], centre.At[1]}, weight})
	}
	return centres
}

// FloorsIn returns the maximum number of floors in a year.
func (scenario *Scenario) FloorsIn(year float64) int {
	for _, limit := range scenario.MaximumFloors {
		if limit.Before == nil || year < *limit.Before {
			return limit.Floors
		}
	}
	return scenario.MaximumFloors[len(scenario.MaximumFloors)-1].Floors
}

// FirstWallYear returns the latest year of the first wall (+Inf when there is none).
func (scenario *Scenario) FirstWallYear() float64 {
	if scenario.FirstWall.Year == nil {
		return math.Inf(1)
	}
	return *scenario.FirstWall.Year
}
