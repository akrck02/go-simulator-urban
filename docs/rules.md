# Rules of the urban plugin

Version 0.1.0, protocol 1. Pre-industrial urban growth, year by year, on real relief (Weber et al. 2009; Conzen 1960; Hillier and Iida 2005).

This reference is generated from what the plugin itself tells the host (the `describe` message), so it always
matches the code. Parameters go under `with:` in a rules file; see go-simulator's
[rules file reference](https://github.com/akrck02/go-simulator/blob/feature/simuladores-genericos/docs/rules-files.md).

| Rule | What it does |
|---|---|
| [`city`](#city) | Sets up a city from its scenario (relief, inputs, population curve, centres, grid, floors, walls) and starts each year. Its instance name names the city. |
| [`densify`](#densify) | Fills existing plots before the city spreads (burgage cycle). |
| [`expand`](#expand) | Lays plots along free frontages and opens new streets when none are left. |
| [`walls`](#walls) | Raises the first wall and new ones when the suburbs grow too big. |
| [`prune_empty_streets`](#prune_empty_streets) | Removes dead ends nobody built on after some years. |
| [`split_large_blocks`](#split_large_blocks) | Cuts an alley across blocks wider than a size (Siksna 1997). |
| [`space_syntax`](#space_syntax) | Angular segment analysis: main streets, the market square and shops. |
| [`indicators`](#indicators) | Emits an "indicators" event with the city's form measures in the listed years (every step when none). |
| [`squares`](#squares) | Opens irregular squares at busy crossings, inside gates and at bridgeheads (scenario "squares"). |

## city

Sets up a city from its scenario (relief, inputs, population curve, centres, grid, floors, walls) and starts each year. Its instance name names the city.

- **Reads:** entities of kind `river`; fields `elevation`
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `relief` | map |  | {path, minimum_x, minimum_z, cell_size}: heights in metres (.npy), rows from the minimum z; without it, the world's elevation field |
| `features` | text |  | GeoJSON with role: window, road, rivers, obstacles, wall_support, clear |
| `population` | map |  | {model: logistic, from_year, from, to_year, to, ceiling} |
| `centres` | list |  | [{at: [x, z], weight, grows_from, grows_for}] |
| `grid` | list |  | [{from_year, angle_degrees, area}] |
| `max_floors` | list |  | [{before, floors}, …, {floors}] |
| `first_wall` | map |  | {year, population} |
| `wall_rise` | map |  | {from, height, years}: the wall grows in height over the years (metres) |
| `wall_heights` | list |  | [[year, metres], …]: the standing wall's height by calendar year; wins over wall_rise |
| `nucleus` | vector |  | [x, z] of the first village's square |
| `new_wall_when_outside` | number | `0.3` |  |
| `square_metres_per_person` | number | `30` |  |
| `seed` | number | `1` |  |
| `height_scale` | number | `1` |  |
| `open_streets_below` | number |  | a town opens new streets before building on frontages less wanted than this (0.3: street villages; 0.5–0.7: compact towns) |
| `rare_floors` | map |  | {floors, share}: only the most wanted share of plots may reach these floors |
| `building_depth` | list |  | [from, to]: houses as deep as from…to times their front (without it, coverage grows 0.4 → 0.7) |
| `organic` | map |  | {segment, wander_degrees, field_snapping, continuation, closing, infill}: curvier organic streets |
| `density` | map |  | {core_front_perches, outer_front_perches, core_depth, outer_depth, core_house_depth, outer_house_depth, outer_floors, radius_factor, back_buildings_from, districts}: density from the core to the outskirts |
| `squares` | map |  | {per_inhabitants, spacing, size, at_gates, at_bridges}: irregular squares at busy crossings, gates and bridgeheads |
| `river` | map |  | {reach, alignment, frontage_bonus, quays, longest_bridge, bridge_chance, bridge_spacing, workshops_by_water, from_world, narrowest}: how rivers shape the town |
| `colours` | map |  | sRGB "#rrggbb" by kind, use or "garden", for viewing without a scene style; the rest is grey |
| `clear_nodes` | list |  | scene nodes cleared under the "clear" features when the city starts |
| `tree_nodes` | list |  | scene nodes of trees, cleared there too and under every new plot |

## densify

Fills existing plots before the city spreads (burgage cycle).

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |

## expand

Lays plots along free frontages and opens new streets when none are left.

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |

## walls

Raises the first wall and new ones when the suburbs grow too big.

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |

## prune_empty_streets

Removes dead ends nobody built on after some years.

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |
| `grace_years` | number | `8` |  |

## split_large_blocks

Cuts an alley across blocks wider than a size (Siksna 1997).

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |
| `maximum_side` | number | `0.12` | world units |

## space_syntax

Angular segment analysis: main streets, the market square and shops.

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |
| `samples` | number | `250` |  |
| `local_radius` | number | `0.8` |  |

## indicators

Emits an "indicators" event with the city's form measures in the listed years (every step when none).

- **Reads:** nothing from other plugins
- **Writes:** nothing (it only emits events)

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |
| `years` | list |  |  |

## squares

Opens irregular squares at busy crossings, inside gates and at bridgeheads (scenario "squares").

- **Reads:** nothing from other plugins
- **Writes:** `street`, `parcel`, `building`, `wall`, `old_wall`, `gate`, `square`, `bridge`

| Parameter | Type | Default | Meaning |
|---|---|---|---|
| `city` | text |  | instance name of the city rule |
