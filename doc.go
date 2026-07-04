// Package graphql is a pure-Go (cgo-free) reimplementation of the public
// surface of Ruby's graphql gem (graphql-ruby): it builds a GraphQL schema from
// a set of root types and executes query documents against it, returning a
// result Hash whose data / errors shape matches graphql-ruby.
//
// # Relationship to graphql-ruby
//
// graphql-ruby's headline API is a large class-macro DSL — types are Ruby
// classes that inherit from GraphQL::Schema::Object, GraphQL::Schema::Enum and
// friends, and fields are declared with the `field` macro. That class-based DSL
// is replaced here by an equivalent programmatic schema builder: the constructor
// functions NewObjectType, NewEnumType, NewInterfaceType, NewUnionType,
// NewInputObjectType and NewScalarType, composed with ListType and NonNullType,
// stand in for the corresponding GraphQL:: classes, and a Field's Resolve
// function stands in for a resolver method. What is preserved faithfully is the
// runtime contract: the type system, query execution semantics, introspection,
// and — crucially — the exact shape of the returned result and of the entries in
// its "errors" array (message, locations, path).
//
// # Consuming the Go ecosystem
//
// Execution is not reimplemented. This package is a thin, Ruby-flavoured façade
// over github.com/graphql-go/graphql, a mature pure-Go implementation of the
// GraphQL specification (lexer, parser, validator, type system, executor and
// introspection). Every type constructor wraps the corresponding graphql-go
// type, and Schema.Execute delegates to graphql-go's executor, then reshapes the
// result into graphql-ruby's Hash form. Because the whole pipeline is pure Go
// with no network dependency, a schema and its queries run entirely in-process.
//
// # The type system
//
// The five built-in scalars are the package values Int, Float, String, Boolean
// and ID; custom scalars are built with NewScalarType (Serialize mirrors
// coerce_result and ParseValue mirrors coerce_input). Object types are built
// with NewObjectType from a FieldMap; each Field has a Type, optional Args
// (each an Argument with an optional DefaultValue) and a Resolve function.
// Interfaces and unions (NewInterfaceType / NewUnionType) use a ResolveType hook
// to map a runtime value to its concrete object type, mirroring graphql-ruby's
// resolve_type. Input objects (NewInputObjectType) provide structured arguments.
//
// # Executing queries
//
//	schema, err := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})
//	result := schema.Execute(`{ hero { name } }`, graphql.ExecuteParams{
//		Variables:     map[string]interface{}{"id": "1000"},
//		OperationName: "HeroQuery",
//	})
//	data := result.Data()     // map[string]interface{}, or nil
//	errs := result.Errors()   // []map[string]interface{}, or nil
//
// Variables, fragments (named and inline), field aliases, the @include and @skip
// directives, and the __schema / __type introspection fields are all supported
// by the underlying executor. A resolver that returns an *ExecutionError (or any
// error) collects a field error at the current path and resolves the field to
// null, exactly as a raised GraphQL::ExecutionError does in graphql-ruby; a
// document that fails to parse or validate yields a result carrying only the
// "errors" array, with no "data" key.
package graphql
