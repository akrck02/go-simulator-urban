"""Draws snapshots of the urban plugin (GeoJSON in ground coordinates x, z) to review them: plots,
buildings by use, streets by class and walls, over the inputs (window, rivers, obstacles, roads).

    uv run --no-project --with matplotlib python tools/view_snapshot.py out.png folder [folder ...]

Every folder holds final.geojson and inputs.geojson (as written by the places test with
URBAN_PLACES_OUTPUT). North is up: z grows towards the south.
"""
import json
import math
import sys
from pathlib import Path

import matplotlib
matplotlib.use("Agg")
import matplotlib.pyplot as plt
from matplotlib.patches import Polygon

COLOURS = {"garden": "#d5d3ad", "extensive": "#9aa37a", "housing": "#b86f52", "shop": "#d08a5c", "workshop": "#6b5a4c",
           "water": "#9cc3d5", "obstacle": "#b7b2a6", "street": "#ffffff", "road": "#e3c26a", "main": "#f2e6c4", "quay": "#cfc6a8",
           "wall": "#3d3526", "old_wall": "#a39a86", "square": "#efe7d1", "bridge": "#3d3526", "window": "#7a6e55"}


def flipped(points):
    return [(x, -z) for x, z in points]


def rings_of(geometry):
    if geometry["type"] == "Polygon":
        return [geometry["coordinates"][0]]
    if geometry["type"] == "MultiPolygon":
        return [polygon[0] for polygon in geometry["coordinates"]]
    return []


def draw_inputs(axis, inputs):
    for feature in inputs["features"]:
        role, geometry = feature["properties"]["role"], feature["geometry"]
        if role in ("rivers", "obstacles"):
            for ring in rings_of(geometry):
                axis.add_patch(Polygon(flipped(ring), facecolor=COLOURS["water" if role == "rivers" else "obstacle"], edgecolor="none", zorder=0))
        elif role == "window":
            for ring in rings_of(geometry):
                xs, zs = zip(*flipped(ring))
                axis.plot(xs, zs, color=COLOURS["window"], linewidth=0.6, linestyle=(0, (4, 3)), zorder=0)


def draw_entity(axis, feature):
    kind, properties, geometry = feature["properties"]["kind"], feature["properties"], feature["geometry"]
    if geometry["type"] == "Polygon":
        if kind == "parcel":
            colour = COLOURS["extensive"] if properties.get("use") == "extensive" else COLOURS["garden"]
        elif kind == "building":
            colour = COLOURS.get(properties.get("use"), COLOURS["housing"])
        else:
            colour = COLOURS.get(kind, "#999999")
        axis.add_patch(Polygon(flipped(geometry["coordinates"][0]), facecolor=colour, edgecolor="none",
                               zorder={"parcel": 1, "square": 2, "building": 4}.get(kind, 3)))
    elif geometry["type"] == "LineString":
        xs, zs = zip(*flipped(geometry["coordinates"]))
        if kind == "river":
            colour, width = COLOURS["water"], 6
        elif kind == "street":
            colour = COLOURS.get(properties.get("class"), COLOURS["street"])
            width = {"road": 1.6, "main": 1.2, "ring": 1.4}.get(properties.get("class"), 0.7)
        elif kind != "river":
            colour, width = COLOURS.get(kind, "#999999"), {"wall": 2.0, "old_wall": 1.0, "bridge": 2.2}.get(kind, 1)
        axis.plot(xs, zs, color=colour, linewidth=width, solid_capstyle="round", zorder=5 if "wall" in kind or kind == "bridge" else 3)


def draw_place(axis, folder):
    inputs = json.loads((folder / "inputs.geojson").read_text())
    final = json.loads((folder / "final.geojson").read_text())
    axis.set_facecolor("#e4e8d6")
    draw_inputs(axis, inputs)
    for feature in final["features"]:
        draw_entity(axis, feature)
    axis.set_aspect("equal")
    axis.set_xlim(-1.6, 1.6)
    axis.set_ylim(-1.6, 1.6)
    axis.set_xticks([])
    axis.set_yticks([])
    axis.set_title(final["properties"].get("place", folder.name), loc="left", fontsize=12)


def main():
    output, folders = Path(sys.argv[1]), [Path(argument) for argument in sys.argv[2:]]
    columns = min(3, len(folders))
    rows = math.ceil(len(folders) / columns)
    figure, axes = plt.subplots(rows, columns, figsize=(6.5 * columns, 6.5 * rows), squeeze=False, facecolor="#f4efe4")
    for axis, folder in zip(axes.flat, folders):
        draw_place(axis, folder)
    for axis in list(axes.flat)[len(folders):]:
        axis.set_axis_off()
    figure.tight_layout()
    figure.savefig(output, dpi=130, facecolor="#f4efe4")
    print(output)


if __name__ == "__main__":
    main()
