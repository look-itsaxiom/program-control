# GraphQL layer (phase 4)

`schema.graphqls` is empty until phase 4. Write your types, queries, and mutations there, then
generate resolvers and models by running, from `api/`:

```sh
go tool gqlgen generate --config internal/graph/gqlgen.yml
```

(The `--config` flag is needed because this config lives in `internal/graph/`, not at the module
root, where `go tool gqlgen generate` looks by default.)

That produces `internal/graph/generated/generated.go` and `internal/graph/model/models_gen.go`,
and scaffolds resolver stubs under `internal/graph/` for you to implement, following the schema
layout configured in `gqlgen.yml`.
