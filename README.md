<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-graphql/brand/main/social/go-ruby-graphql-graphql.png" alt="go-ruby-graphql/graphql" width="720"></p>

# graphql — go-ruby-graphql

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![CGO](https://img.shields.io/badge/cgo-0-1a7f37)](doc.go)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](.github/workflows/ci.yml)

**A pure-Go (no cgo) reimplementation of the public surface of Ruby's
[`graphql`](https://graphql-ruby.org) gem (graphql-ruby)** — build a GraphQL
schema from a set of root types and execute query documents against it, getting
back a result whose **`data` / `errors` shape matches graphql-ruby** exactly.

It is a schema-and-execution façade for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but a
**standalone, reusable** module with no dependency on the Ruby runtime, and
entirely self-contained: a schema and its queries run in-process, no network.

## What it consumes

Execution is **not** reimplemented. This package is a thin, Ruby-flavoured
façade over [`github.com/graphql-go/graphql`](https://github.com/graphql-go/graphql),
a mature pure-Go implementation of the GraphQL specification — lexer, parser,
validator, type system, executor and introspection. Every type constructor
wraps the corresponding graphql-go type, and `Schema.Execute` delegates to the
graphql-go executor, then reshapes the result into graphql-ruby's Hash form.

## Relationship to graphql-ruby

graphql-ruby's headline API is a large **class-macro DSL** — types are Ruby
classes inheriting from `GraphQL::Schema::Object` and friends, with fields
declared by the `field` macro. That class-based DSL is replaced here by an
**equivalent programmatic schema builder**: the `New…Type` constructors stand
in for the corresponding `GraphQL::` classes, and a field's `Resolve` function
stands in for a resolver method. What is preserved faithfully is the runtime
contract — the type system, execution semantics, introspection, and the exact
shape of the result and its `errors` entries (`message`, `locations`, `path`).

| graphql-ruby | this package |
| --- | --- |
| `GraphQL::Schema` + `.execute(query, variables:, context:, operation_name:)` | `NewSchema` + `Schema.Execute(query, ExecuteParams{…})` |
| `GraphQL::ObjectType` / `field` / `argument` | `NewObjectType` / `Field` / `Argument` |
| `Int`/`Float`/`String`/`Boolean`/`ID` | `Int` `Float` `String` `Boolean` `ID` |
| custom scalar (`coerce_input`/`coerce_result`) | `NewScalarType` (`ParseValue`/`Serialize`) |
| `GraphQL::EnumType` | `NewEnumType` |
| `GraphQL::InterfaceType` + `resolve_type` | `NewInterfaceType` + `ResolveType` |
| `GraphQL::UnionType` + `resolve_type` | `NewUnionType` + `ResolveType` |
| `GraphQL::InputObjectType` | `NewInputObjectType` |
| `[T]` / `T!` | `ListType(T)` / `NonNullType(T)` |
| `GraphQL::ExecutionError` | `ExecutionError` / `NewExecutionError` |

## Usage

```go
queryType := graphql.NewObjectType(graphql.ObjectTypeConfig{
    Name: "Query",
    Fields: graphql.FieldMap{
        "hero": {
            Type: graphql.String,
            Resolve: func(p graphql.ResolveParams) (interface{}, error) {
                return "R2-D2", nil
            },
        },
    },
})

schema, _ := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})

result := schema.Execute(`{ hero }`)
result.Data()   // map[string]interface{}{"hero": "R2-D2"}
result.Errors() // nil
```

Variables, fragments (named and inline), field aliases, the `@include` / `@skip`
directives, and the `__schema` / `__type` introspection fields are all
supported. A resolver returning an `*ExecutionError` (or any error) collects a
field error at the current path and nulls the field, exactly as a raised
`GraphQL::ExecutionError` does in graphql-ruby; a document that fails to parse
or validate yields a result carrying only `errors`, with no `data` key.

## Conformance

- **Pure Go, `CGO=0`** — no C dependencies.
- **100% test coverage**, `-race` clean.
- **6 × 64-bit arches** — amd64, arm64, riscv64, loong64, ppc64le, and
  **s390x (big-endian)**, the last four under qemu-user in CI.
- **3 OSes** — Linux, macOS, Windows.
- **BSD-3-Clause** licensed.

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-graphql/graphql authors.
