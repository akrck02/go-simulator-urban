# 🗺️ The city's scenario

The scenario is the `with:` of the `city` rule, or files it includes
(`with: {include: [derived.yaml, town.yaml]}`). Coordinates are world coordinates on the ground
plane (x, z) in kilometres; relative paths start at the rules file's folder. Keys the plugin does
not know are ignored, so a scenario can carry notes for people.

```yaml
- rule: city
  name: my_town
  with:
    features: inputs.geojson
    nucleus: [0, 0]
    population: {model: logistic, from_year: 0, from: 300, to_year: 100, to: 6000, ceiling: 8000}
    centres: [{at: [0, 0], weight: 1}, {at: [0.3, 0.8], weight: 0.9, grows_from: 30, grows_for: 40}]
    grid: [{from_year: 40, angle_degrees: 20, area: [[x, z], …], block: 0.08}]
    open_streets_below: 0.3
    max_floors: [{before: 40, floors: 2}, {before: 80, floors: 3}, {floors: 4}]
    rare_floors: {floors: 3, share: 0.05}
    building_depth: [1, 1.5]
    first_wall: {year: 60, population: 3000}
    wall_heights: [[60, 4], [70, 4], [100, 25]]
    new_wall_when_outside: 0.3
    square_metres_per_person: 30
    seed: 7
    height_scale: 1
```

## Keys

| Key | Default | Meaning |
|---|---|---|
| `relief` | the world's `elevation` | `{path, minimum_x, minimum_z, cell_size}`: a `.npy` of heights in metres, rows from the smallest z, placed by its first cell centre |
| `features` | — | GeoJSON inputs (see below) |
| `nucleus` | — | `[x, z]` of the first village's square |
| `population` | required | `{model: logistic, from_year, from, to_year, to, ceiling}`; the ceiling must be above `to` |
| `centres` | required | `[{at: [x, z], weight, grows_from, grows_for}]`: points that pull the city; a centre can start in a year and grow over some years |
| `grid` | none | `[{from_year, angle_degrees, area, block}]`: zones laid out on a lattice of `block` km (0.08) |
| `open_streets_below` | 0.3 | 0.3: street villages along the roads; 0.5–0.7: compact towns with back streets |
| `max_floors` | `[{floors: 3}]` | `[{before: year, floors}, …, {floors}]`: the most floors in each period |
| `rare_floors` | none | `{floors, share}`: only the most wanted share of plots may reach these floors |
| `building_depth` | coverage 0.4 → 0.7 | `[from, to]`: houses as deep as from…to times their front |
| `first_wall` | population 3000 | `{year, population}`: the first wall goes up at the population or, at the latest, in the year |
| `wall_rise` | 11 m from the start | `{from, height, years}`: metres the year it goes up, then evenly up to `height` |
| `wall_heights` | none | `[[year, metres], …]`: the standing wall's height by calendar year; wins over `wall_rise` |
| `new_wall_when_outside` | 0.3 | share of the floor space outside the wall that raises a new one |
| `square_metres_per_person` | 30 | floor space a person needs |
| `seed` | 1 | the same seed grows the same city |
| `height_scale` | 1 | how much buildings are exaggerated in 3D |
| `clear_nodes` | none | scene nodes cleared under the `clear` features at the start |
| `tree_nodes` | none | scene nodes of trees, cleared there too and under every new plot |
| `colours` | grey | sRGB `"#rrggbb"` by kind, use or `"garden"`, for viewing without a scene style |

### `organic`

Curvier organic streets.

| Key | Default | Meaning |
|---|---|---|
| `segment` | 0.055 | mean length of a street piece (km) |
| `wander_degrees` | 12 | spread of the turn at every piece |
| `field_snapping` | 1 | chance a street bends to the tensor field when close to it |
| `continuation` | 1 | how much more likely a street goes on from its end than branches |
| `closing` | 1.5 | how far ahead (× the piece) a street looks to close a block |
| `infill` | 0 | extra want of frontages among built plots, so streets fill before new ones open |

### `density`

From the core to the outskirts.

| Key | Default | Meaning |
|---|---|---|
| `core_front_perches`, `outer_front_perches` | [1, 1.5], [3, 5] | plot fronts, in perches (5.03 m) |
| `core_depth`, `outer_depth` | [0.025, 0.04], [0.035, 0.05] | plot depths (km) |
| `core_house_depth`, `outer_house_depth` | 2, 1 | deepest house, × the front |
| `outer_floors` | 1 | most floors in the outskirts |
| `radius_factor` | 0.6 | the core's radius, × the growth radius |
| `back_buildings_from` | 0 (never) | density from which full plots build at the back, round a yard |
| `districts` | none | `[{name, area, density}]`: areas with their own density |

### `squares`

Irregular squares at busy crossings, gates and bridgeheads: `per_inhabitants` (2500), `spacing` (0.2 km), `size` ([0.012, 0.03] km of reach
from the crossing).

### `river`

How rivers shape the town: `reach` (0.25 km), `alignment` (1, weight of the banks in street directions),
`frontage_bonus` (1), `quays` (true), `longest_bridge` (1.6 × the river's width), `bridge_chance`
(0.6), `bridge_spacing` (0.15 km), `workshops_by_water` (0.3).

## Input features

`features` is a GeoJSON collection; every feature has a `role`:

| Role | Geometry | Meaning |
|---|---|---|
| `window` | polygon | where the city may grow (required) |
| `road` | line | roads that already exist; the city grows along them |
| `river` | line, with `width` (km) | a river that crosses the town (see the model) |
| `rivers` | polygon | other water, an obstacle |
| `obstacles` | polygon | where nothing is built |
| `wall_support` | polygon | rock the wall ties to instead of crossing it |
| `wall_outline` | polygon | the shape of the wall, when it is given; then it is the only wall |
| `clear` | polygon | where the drawn scene is cleared at the start |
