# go-simulator-urban

Town growth plugin for go-simulator: streets, plots, densification, walls, squares and space syntax for pre-industrial towns. Start with `docs/5-the-model.md`; parameters in `docs/8-rules-reference.md`.

## Commands

```bash
go vet ./...
go test ./...
cd ../go-simulator && go run ./cmd/simulate -rules <rules.yaml> -output results/
```

Sibling checkouts are required: `go.mod` replaces the akrck02 modules with `../go-simulator`,
`../go-renderer` and the like, so clone them next to this one.

## Rules

- **Generic only.** This project knows nothing about any particular world, game or story. Never put
  names, places, dates, numbers or settings from a user's own project in code, comments, tests,
  examples, docs or commit messages. Examples use made-up places (like the example island in the docs)
  and round, neutral numbers.
- **Readable code.** Full-word names (no acronyms, no one-letter variables), small functions that do
  one thing, early returns instead of nested branches, comments that say *why*. Match the code
  around you.
- **Tests with every change.** `go vet ./...` and `go test ./...` must pass before committing.
- **Docs live in `docs/`**, written for people: explanatory, friendly, with 2D images where they
  help. Keep the README short and point to `docs/`. Update the docs in the same commit as the
  behaviour they describe.
- **AI disclosure.** Commits written with an AI assistant end with a `Co-Authored-By: Claude …`
  line, and the *AI disclosure* section of the README stays true. The
  code is pending human review: say so, don't overclaim.
- **Git.** Commit only when asked; never push, force-push, rewrite history or delete branches or
  the repository without the author's explicit permission. Do not change the global git config.

## Plugin contract

- A plugin never touches the renderer: it reads its copy of the world and answers with changes
  (see go-simulator's `docs/4-guides/4-writing-plugins.md`).
- Fields use physical units and shapes use world units, as listed in go-simulator's
  `docs/5-reference/1-vocabulary-and-conditions.md`. Read what other plugins publish from the world instead of loading it again.
- Every number of the model comes from a published source (cite it in `docs/5-the-model.md`) or from the
  scenario/rules file; nothing about a particular place is hard-coded.
- `docs/8-rules-reference.md` is generated from the plugin's `describe` message: when a rule or a parameter
  changes, regenerate it so it stays in step with the code.
