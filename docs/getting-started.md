# 🚀 Getting started

## The folders it needs

The plugin is developed next to the projects it uses: `go.mod` replaces go-simulator and
go-renderer with the folders beside this one. Clone them side by side:

```
go-renderer/
go-simulator/
go-simulator-urban/   ← this repository
```

## GEOS

The plugin cuts plots, walls and squares with GEOS, the geometry engine shapely uses. Install it
before building:

```bash
brew install geos                # macOS
sudo apt install libgeos-dev     # Debian, Ubuntu
sudo dnf install geos-devel      # Fedora
```

`pkg-config geos` should then print its version. While it is not installed, a `.geos-local/env.sh`
(ignored by git) can point cgo at an unpacked copy: `. .geos-local/env.sh` before building.

## A whole rules file

The `city` rule sets up a city and opens every year; its instance name names the city, and the
other rules find it by their `city` parameter, so several cities can grow in one simulation. This is
the made-up river town drawn in these pages:

```yaml
plugins:
  - name: urban
    command: [go, run, .]
    directory: ../go-simulator-urban       # where the plugin runs

stages:
  - name: city
    clock: {unit: year, from: -1, to: 100, step: 1}
    rules:
      - rule: city
        name: river_town
        with:
          relief: {path: relief.npy, minimum_x: -1.6, minimum_z: -1.6, cell_size: 0.0125}
          features: inputs.geojson
          seed: 7
          nucleus: [0, -0.12]
          population: {model: logistic, from_year: 0, from: 600, to_year: 100, to: 36000, ceiling: 42000}
          open_streets_below: 0.5
          centres: [{at: [0, -0.12], weight: 1}, {at: [0.15, 0.35], weight: 0.6, grows_from: 30, grows_for: 30}]
          max_floors: [{before: 40, floors: 2}, {before: 80, floors: 3}, {floors: 4}]
          first_wall: {population: 4000}
          river: {reach: 0.25}
          squares: {per_inhabitants: 2500}
      - {rule: densify, with: {city: river_town}}
      - {rule: expand, with: {city: river_town}}
      - {rule: walls, with: {city: river_town}}
      - {rule: prune_empty_streets, with: {city: river_town}, every: 5}
      - {rule: split_large_blocks, with: {city: river_town}, every: 5}
      - {rule: space_syntax, with: {city: river_town, samples: 200}, every: 5}
      - {rule: squares, with: {city: river_town}, every: 5}
      - {rule: indicators, with: {city: river_town}}
```

`inputs.geojson` holds the window the town may grow in (a 3 × 3 km square), a river 35 m wide,
three roads that meet near the river and an obstacle where nothing is built:

| Role | Geometry |
|---|---|
| `window` | the square from `[-1.5, -1.5]` to `[1.5, 1.5]` |
| `river` (`width: 0.035`) | from `[-1.6, 0.25]` through `[-0.6, 0.12]`, `[-0.1, 0.05]`, `[0.4, 0.12]`, `[1, 0]` to `[1.6, 0.05]` |
| `road` | from `[-1.6, -0.35]` through `[-0.6, -0.2]`, `[0, -0.12]`, `[0.7, -0.25]` to `[1.6, -0.15]` |
| `road` | from `[0.1, -1.6]` through `[0.05, -0.6]`, `[0, -0.12]`, `[0.05, 0.4]` to `[0.25, 1.6]` |
| `road` | from `[0, -0.12]` through `[-0.5, 0.6]` to `[-1.2, 1.6]` |
| `obstacles` | the quadrilateral `[0.8, 0.6]`, `[1.3, 0.55]`, `[1.4, 1]`, `[0.9, 1.1]` |

`relief.npy` is a grid of heights in metres: a gentle plain, a little lower along the river, with a
low hill to the north-west. Without a `relief`, the city reads the world's `elevation` field, so the
terrain plugin can go first instead. Every key is in [the scenario](scenario.md).

## Running it

Without a window, to the end of the clock, saving what the stages ask for:

```bash
cd ../go-simulator
go run ./cmd/simulate -rules <rules.yaml> -output results/
```

With go-simulator's viewer, over a glTF scene of the ground, which the town is drawn on as it grows:

```bash
go run ./cmd/simulator -rules <rules.yaml> scene.glb
```

go-simulator's [running guide](https://github.com/akrck02/go-simulator/blob/main/docs/running.md)
has the rest: snapshots, objectives and the viewer's keys.

## The tests

```bash
go vet ./...
go test ./...
```

[Testing](testing.md) has the six synthetic places the tests grow, and how to draw them.
