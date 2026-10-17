<p align="center"><img alt="" src="images/logo.svg" width="120"></p>

# Welcome to go-simulator-urban 👋

**The urban plugin grows towns**, year after year: a village where roads meet becomes a town with streets of its own, squares, bridges and walls.

<p align="center"><img alt="A town growing over a century" src="images/town_growth.jpg" width="720"></p>

You give it a place (the area it may use, the roads and rivers already there) and how many people live there each year; it finds room for them plot by plot where people would most want to live, and builds streets, squares and walls as the town needs them.

## Is it for me?

- ✅ You use [go-simulator](https://github.com/akrck02/go-simulator) and want **towns that grow**.
- ✅ You want to **set it up by writing a rules file**: no programming needed.
- ✅ You are curious about **the science** behind it: the pages explain the model with pictures.

## Where to start

Pick the line that sounds like you.

| If you… | Read | Time |
|---|---|---|
| are **new** and want to see it work | 🏴‍☠️ [The quest](2-the-quest.md): a market town that grows for a century, in 5 levels | 45 min |
| want to **run its example** | [Getting started](3-getting-started.md) | 10 min |
| want to **set up a town** of your own | [Writing a scenario](4-writing-a-scenario.md) | 15 min |
| are **looking up** a parameter | [Rules reference](8-rules-reference.md) | — |

## All the pages, in reading order

The menu on the left (or the files in `docs/`) follows this order.

**Start**

- [Overview](1-overview.md): this page
- [The quest](2-the-quest.md)
- [Getting started](3-getting-started.md)

**Understand**

- [Writing a scenario](4-writing-a-scenario.md)
- [The model](5-the-model.md)
- [What it writes](6-what-it-writes.md)
- [How it works](7-how-it-works.md)
- [Testing](9-testing.md)

**Reference**

- [Rules reference](8-rules-reference.md)

## 🚧 Not done yet

- Pictures of the 3D viewer itself. The pictures in these pages are 2D drawings of what the plugin
  answered, not screenshots (see [how the pictures were made](7-how-it-works.md#how-the-pictures-were-made)).
- Gates in the pictures. The plugin publishes the standing wall's gates as `gate` points (see
  [outputs](6-what-it-writes.md)), but the pictures were drawn before, so they do not show them.
- Grid zones, districts and walls tied to rock are described in the [model](5-the-model.md) and the
  [scenario](4-writing-a-scenario.md), but the example town in the pictures uses none of them.

## Words you will meet

| Word | What it means |
|---|---|
| Terminal | The window where you type commands: *Terminal* on macOS and Linux, *PowerShell* on Windows. |
| Repository | A project's folder of files, kept on GitHub. `git clone <address>` copies it to your computer. |
| Plugin | A small program that adds one ability to another: here, one kind of simulation each. |
| Rules file | A text file (YAML) that says which plugins to use, with which numbers, for how long: no programming needed. |
| Field | A grid of numbers laid over the world: the height of every spot, its temperature, its rain. |
| Quest | The tutorial of these docs: short levels, each ending with something that works and an achievement. |

## The family

go-simulator-urban is one of a family of small projects that work together:

| Project | What it does |
|---|---|
| [go-renderer](https://github.com/akrck02/go-renderer) | draws 3D scenes and flat interfaces |
| [go-ui](https://github.com/akrck02/go-ui) | buttons, panels and whole applications |
| [go-ui-android](https://github.com/akrck02/go-ui-android) | Android apps that look like Android's own |
| [go-sound](https://github.com/akrck02/go-sound) | sound and music |
| [go-scene-builder](https://github.com/akrck02/go-scene-builder) | 3D scenes from a flat map |
| [go-simulator](https://github.com/akrck02/go-simulator) | simulations that change a scene |
| [go-simulator-terrain](https://github.com/akrck02/go-simulator-terrain) | ground shaped by rivers |
| [go-simulator-climate](https://github.com/akrck02/go-simulator-climate) | weather and biomes |
| [go-simulator-flora](https://github.com/akrck02/go-simulator-flora) | forests that grow |
| **go-simulator-urban** (this one) | towns that grow |
| [go-simulator-motion](https://github.com/akrck02/go-simulator-motion) | a scene that moves |
| [go-3ds](https://github.com/akrck02/go-3ds) | Go on the Nintendo 3DS |

> ⚠️ All of them are **educational experiments**, written with an AI assistant and pending review by their author: good for learning and playing, not for production.
