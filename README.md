# go-simulator-urban

A [go-simulator](../go-simulator) plugin that grows pre-industrial cities year by year on real
relief: streets, burgage plots, densification, walls and space syntax. Every rule comes from a
published model (see the references in `urban/indicators.go` and in each rule); the only rule of its
own is the walls', because no published model simulates them.

It started as a port of a Python prototype: on the same scenario both give the same city within
the randomness, and this one runs in under a minute instead of four.

## Rules

| Rule | What it does | Parameters |
|---|---|---|
| `city` | Sets up a city from its scenario and starts every year (demand and deficit). Its instance `name` names the city | see below |
| `densify` | Fills existing plots before the city spreads: coverage up to 0.7, then floors (Conzen's burgage cycle) | `city` |
| `expand` | Lays plots along the most wanted free frontages; when none are left, opens streets (Weber et al. 2009, with a tensor field after Chen et al. 2008) | `city` |
| `walls` | Raises the first wall at a population or year, and a new one when the suburbs hold too much; the old one becomes a ring road with a belt | `city` |
| `prune_empty_streets` | Removes dead ends nobody built on | `city`, `grace_years` (8) |
| `split_large_blocks` | Cuts an alley across blocks wider than a size (Siksna 1997) | `city`, `maximum_side` (0.12) |
| `space_syntax` | Angular segment analysis (Hillier and Iida 2005): main streets, the market square, shops | `city`, `samples` (250), `local_radius` (0.8) |
| `indicators` | Emits an `indicators` event with the form measures to compare with historic towns | `city`, `years` |

The city rule's scenario:

```yaml
- rule: city
  name: my_town
  with:
    relief: {path: relief.npy, minimum_x: -1, minimum_z: -1, cell_size: 0.0125}  # heights in metres, rows from the minimum z
    features: inputs.geojson     # features with "role": window, road, rivers, obstacles, wall_support, clear
    nucleus: [0, 0]              # the first village's square
    population: {model: logistic, from_year: 0, from: 300, to_year: 100, to: 6000, ceiling: 8000}
    centres: [{at: [0, 0], weight: 1}, {at: [0.3, 0.8], weight: 0.9, grows_from: 30, grows_for: 40}]
    grid: [{from_year: 40, angle_degrees: 20, area: [[x, z], ...]}]
    max_floors: [{before: 40, floors: 2}, {before: 80, floors: 3}, {floors: 4}]
    first_wall: {year: 60, population: 3000}
    new_wall_when_outside: 0.3
    square_metres_per_person: 30
    seed: 7
    height_scale: 3              # exaggeration of building heights in 3D
    clear_nodes: [buildings, grounds, paths]   # scene nodes cleared under the "clear" features at the start
    tree_nodes: [broadleaf forest, laurel forest] # trees, cleared there too and under every new plot
```

Coordinates are world coordinates on the ground plane (x, z) in kilometres; relative paths start at
the rules file's folder. A stepped clock with a `pace` shows the city growing in the viewer:
`clock: {unit: year, from: 0, to: 100, step: 1, pace: 0.4}`.

When the drawn scene already has a town where the city grows, a `clear` feature marks it: at the
start the plugin sends a `clear_area` for the `clear_nodes` (holes in the feature are spared), and
every year it clears the `tree_nodes` under the new plots.

The city appears as entities with ground shapes, one batch per kind: `street` (lines with their
class), `parcel`, `building` (volumes, floors × 3.2 m + 2 m), `wall`, `old_wall`,
`square` and `bridge`. Their properties (use, coverage, floors, front, depth, class, year…) travel
with them, so snapshots can be measured or drawn by other tools.

## Requirements

GEOS, the geometry engine shapely uses: `sudo dnf install geos-devel` on Fedora, `brew install geos`
on macOS. While it is not installed, `.geos-local/env.sh` (ignored by git) can point cgo at a copy
unpacked from Fedora's packages: `. .geos-local/env.sh` before building.

## Tests and profiling

```bash
go test ./...
URBAN_RULES=path/to/rules.yaml URBAN_UNTIL=-20 go test -run RulesFile -cpuprofile cpu.out ./urban
```
