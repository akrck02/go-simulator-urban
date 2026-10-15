# 📚 go-simulator-urban documentation

| | Page | What is in it |
|---|---|---|
| 🚀 | [Getting started](getting-started.md) | The folders it needs, GEOS, a whole rules file for a made-up river town, and running it with or without a window. |
| 🏘️ | [The model](model.md) | How a city grows every year: demand, densification, frontages and streets, walls, squares, space syntax, rivers, with the published models behind each rule and pictures of each. |
| 🗺️ | [The scenario](scenario.md) | Every key of the city's scenario, with its default, and the input features. |
| 📤 | [Outputs](outputs.md) | The entities, properties, events and indicators the city publishes. |
| 🧪 | [Testing](testing.md) | The six synthetic places, what is checked on every one, and profiling. |
| 🔍 | [How it works](how-it-works.md) | How the plugin fits go-simulator, what travels each year, and how the pictures were made. |
| 📋 | [Rules reference](rules.md) | Every rule and parameter, generated from the plugin's own description. |

New here? Start with the [project README](../README.md).

## 🚧 Not done yet

- Pictures of the 3D viewer itself. The pictures in these pages are 2D drawings of what the plugin
  answered, not screenshots (see [how the pictures were made](how-it-works.md#how-the-pictures-were-made)).
- Gates as entities. The plugin finds the gates where roads and main streets cross a wall and opens
  squares just inside them, but it does not publish them, although `gate` is in the
  [rules reference](rules.md)'s list of what the rules write. So they are not in the pictures either.
- Grid zones, districts and walls tied to rock are described in the [model](model.md) and the
  [scenario](scenario.md), but the example town in the pictures uses none of them.
