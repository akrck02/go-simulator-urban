package urban

import (
	"encoding/json"
	"fmt"
	"slices"

	"github.com/akrck02/go-simulator/plugin"
	"github.com/akrck02/go-simulator/protocol"
)

// Version of the plugin.
const Version = "0.1.0"

// cities are the cities the plugin simulates, by the instance name of their city rule.
type cities struct {
	byName map[string]*City
	order  []string
}

func (registry *cities) find(name string) (*City, error) {
	city, found := registry.byName[name]
	if !found {
		return nil, fmt.Errorf("no city %q: a \"city\" rule named so must come first", name)
	}
	return city, nil
}

// Plugin returns the urban growth plugin with all its rules.
func Plugin() *plugin.Plugin {
	registry := &cities{byName: map[string]*City{}}
	return &plugin.Plugin{
		Name:    "urban",
		Version: Version,
		Summary: "Pre-industrial urban growth, year by year, on real relief (Weber et al. 2009; Conzen 1960; Hillier and Iida 2005).",
		Rules: []plugin.Definition{
			cityRule(registry), yearlyRule(registry, "densify", "Fills existing plots before the city spreads (burgage cycle).", (*City).densify),
			yearlyRule(registry, "expand", "Lays plots along free frontages and opens new streets when none are left.", (*City).expand),
			yearlyRule(registry, "walls", "Raises the first wall and new ones when the suburbs grow too big.", (*City).wallsIfDue),
			pruneRule(registry), splitRule(registry), syntaxRule(registry), indicatorsRule(registry),
			yearlyRule(registry, "squares", "Opens irregular squares at busy crossings, inside gates and at bridgeheads (scenario \"squares\").", (*City).openSquares),
		},
		AfterStep: func(context *plugin.Context) error {
			for _, name := range registry.order {
				registry.byName[name].publish(context)
			}
			return nil
		},
	}
}

var writesCity = []string{"street", "parcel", "building", "wall", "old_wall", "gate", "square", "bridge"}

func cityParameter() protocol.ParameterDescription {
	return protocol.ParameterDescription{Name: "city", Type: "text", Summary: "instance name of the city rule"}
}

// ---------------------------------------------------------------- city

type cityRuleInstance struct {
	registry *cities
	scenario *Scenario
}

func cityRule(registry *cities) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{
			Name:    "city",
			Summary: "Sets up a city from its scenario (relief, inputs, population curve, centres, grid, floors, walls) and starts each year. Its instance name names the city.",
			Parameters: []protocol.ParameterDescription{
				{Name: "relief", Type: "map", Summary: "{path, minimum_x, minimum_z, cell_size}: heights in metres (.npy), rows from the minimum z"},
				{Name: "features", Type: "text", Summary: "GeoJSON with role: window, road, rivers, obstacles, wall_support, clear"},
				{Name: "population", Type: "map", Summary: "{model: logistic, from_year, from, to_year, to, ceiling}"},
				{Name: "centres", Type: "list", Summary: "[{at: [x, z], weight, grows_from, grows_for}]"},
				{Name: "grid", Type: "list", Summary: "[{from_year, angle_degrees, area}]"},
				{Name: "max_floors", Type: "list", Summary: "[{before, floors}, …, {floors}]"},
				{Name: "first_wall", Type: "map", Summary: "{year, population}"},
				{Name: "nucleus", Type: "vector", Summary: "[x, z] of the first village's square"},
				{Name: "new_wall_when_outside", Type: "number", Default: 0.3},
				{Name: "square_metres_per_person", Type: "number", Default: 30},
				{Name: "seed", Type: "number", Default: 1},
				{Name: "height_scale", Type: "number", Default: 1},
				{Name: "clear_nodes", Type: "list", Summary: "scene nodes cleared under the \"clear\" features when the city starts"},
				{Name: "tree_nodes", Type: "list", Summary: "scene nodes of trees, cleared there too and under every new plot"},
			},
			Writes: writesCity,
		},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			scenario, err := scenarioFrom(parameters)
			if err != nil {
				return nil, err
			}
			return &cityRuleInstance{registry: registry, scenario: scenario}, nil
		},
	}
}

func (rule *cityRuleInstance) Start(context *plugin.Context) error {
	city, err := newCity(context.Instance, rule.scenario)
	if err != nil {
		return err
	}
	if _, exists := rule.registry.byName[city.Name]; !exists {
		rule.registry.order = append(rule.registry.order, city.Name)
	}
	rule.registry.byName[city.Name] = city
	city.clearOldScene(context)
	return nil
}

func (rule *cityRuleInstance) Step(context *plugin.Context) error {
	city, err := rule.registry.find(context.Instance)
	if err != nil {
		return err
	}
	city.startYear(context.Time)
	return nil
}

// ---------------------------------------------------------------- rules that act on a city

type cityAction struct {
	registry *cities
	cityName string
	act      func(city *City)
}

func (rule *cityAction) Step(*plugin.Context) error {
	city, err := rule.registry.find(rule.cityName)
	if err != nil {
		return err
	}
	rule.act(city)
	return nil
}

func yearlyRule(registry *cities, name, summary string, act func(*City)) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{Name: name, Summary: summary, Parameters: []protocol.ParameterDescription{cityParameter()}, Writes: writesCity},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			return &cityAction{registry: registry, cityName: parameters.Text("city", ""), act: act}, nil
		},
	}
}

func pruneRule(registry *cities) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{Name: "prune_empty_streets", Summary: "Removes dead ends nobody built on after some years.",
			Parameters: []protocol.ParameterDescription{cityParameter(), {Name: "grace_years", Type: "number", Default: 8}}, Writes: writesCity},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			grace := parameters.Number("grace_years", 8)
			return &cityAction{registry: registry, cityName: parameters.Text("city", ""), act: func(city *City) { city.pruneEmptyStreets(grace) }}, nil
		},
	}
}

func splitRule(registry *cities) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{Name: "split_large_blocks", Summary: "Cuts an alley across blocks wider than a size (Siksna 1997).",
			Parameters: []protocol.ParameterDescription{cityParameter(), {Name: "maximum_side", Type: "number", Default: 0.12, Summary: "world units"}}, Writes: writesCity},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			side := parameters.Number("maximum_side", 0.12)
			return &cityAction{registry: registry, cityName: parameters.Text("city", ""), act: func(city *City) { city.splitLargeBlocks(side) }}, nil
		},
	}
}

func syntaxRule(registry *cities) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{Name: "space_syntax", Summary: "Angular segment analysis: main streets, the market square and shops.",
			Parameters: []protocol.ParameterDescription{cityParameter(), {Name: "samples", Type: "number", Default: 250}, {Name: "local_radius", Type: "number", Default: 0.8}},
			Writes:     writesCity},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			samples, radius := int(parameters.Number("samples", 250)), parameters.Number("local_radius", 0.8)
			return &cityAction{registry: registry, cityName: parameters.Text("city", ""), act: func(city *City) { city.applySyntax(samples, radius) }}, nil
		},
	}
}

// ---------------------------------------------------------------- indicators

type indicatorsRuleInstance struct {
	registry *cities
	cityName string
	years    []float64
}

func indicatorsRule(registry *cities) plugin.Definition {
	return plugin.Definition{
		Description: protocol.RuleDescription{Name: "indicators", Summary: "Emits an \"indicators\" event with the city's form measures in the listed years (every step when none).",
			Parameters: []protocol.ParameterDescription{cityParameter(), {Name: "years", Type: "list"}}},
		New: func(parameters plugin.Parameters) (plugin.Rule, error) {
			rule := &indicatorsRuleInstance{registry: registry, cityName: parameters.Text("city", "")}
			list, _ := parameters["years"].([]any)
			for _, year := range list {
				if number, isNumber := year.(float64); isNumber {
					rule.years = append(rule.years, number)
				}
			}
			return rule, nil
		},
	}
}

func (rule *indicatorsRuleInstance) Step(context *plugin.Context) error {
	if len(rule.years) > 0 && !slices.Contains(rule.years, context.Time) {
		return nil
	}
	city, err := rule.registry.find(rule.cityName)
	if err != nil {
		return err
	}
	encoded, err := json.Marshal(city.Measure())
	if err != nil {
		return err
	}
	var data map[string]any
	if err := json.Unmarshal(encoded, &data); err != nil {
		return err
	}
	data["city"] = city.Name
	context.Emit("indicators", data)
	return nil
}
