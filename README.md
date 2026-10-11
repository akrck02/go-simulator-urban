# go-simulator-urban

> **Note:** this project was written with an AI assistant and is pending thorough review. See the [AI disclosure](#ai-disclosure).

![A town growing for 100 years on a made-up island](docs/img/town_growth.jpg)

A [go-simulator](https://github.com/akrck02/go-simulator) plugin that grows pre-industrial cities year by year on real
relief: streets, burgage plots, densification, walls, squares and space syntax. Every rule comes
from a published model (Conzen 1960; Weber et al. 2009; Hillier and Iida 2005; Siksna 1997…); the
only rule of its own is the walls', because no published model simulates them.

```yaml
- rule: city
  name: my_town
  with: {include: town_scenario.yaml}
- {rule: densify, with: {city: my_town}}
- {rule: expand, with: {city: my_town}}
- {rule: walls, with: {city: my_town}}
- {rule: indicators, with: {city: my_town, years: [50, 100]}}
```

Documentation in [docs/](docs/README.md): [the model](docs/model.md), [the scenario](docs/scenario.md),
[outputs](docs/outputs.md), [testing](docs/testing.md) and the [rules reference](docs/rules.md).
Needs GEOS (`pkg-config geos`).

## AI disclosure

I write most of my code by hand, so I want to be clear about this project: **it was written by an
AI assistant (Claude, by Anthropic) under my direction.** I decided what to build and how it should
behave; the code, the tests and the documentation were written by the AI. Every commit made this
way carries a `Co-Authored-By: Claude` line (14 of 14 commits here).

**Status: pending thorough review.** The code compiles and its tests pass, but I have not yet
reviewed it line by line. Treat it as a working draft, not as code I vouch for.
