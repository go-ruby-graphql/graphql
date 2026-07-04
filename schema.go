package graphql

import (
	"context"

	gql "github.com/graphql-go/graphql"
)

// Schema mirrors GraphQL::Schema: a validated type system rooted at a query
// type (and optionally mutation and subscription types) against which query
// documents are executed.
type Schema struct{ schema gql.Schema }

// SchemaConfig configures a schema. Query is required; Mutation and
// Subscription are optional root types. Types lists extra types that are not
// reachable from the roots by field traversal (for example the concrete members
// of a union that is only returned via an interface), mirroring graphql-ruby's
// orphan_types / extra type registration.
type SchemaConfig struct {
	Query        *ObjectType
	Mutation     *ObjectType
	Subscription *ObjectType
	Types        []Type
}

// NewSchema builds a schema from its root types. It returns an error when the
// type system is invalid (for example a missing query root or a malformed
// type), mirroring the failure raised by GraphQL::Schema at definition time.
func NewSchema(cfg SchemaConfig) (*Schema, error) {
	sc := gql.SchemaConfig{}
	if cfg.Query != nil {
		sc.Query = cfg.Query.obj
	}
	if cfg.Mutation != nil {
		sc.Mutation = cfg.Mutation.obj
	}
	if cfg.Subscription != nil {
		sc.Subscription = cfg.Subscription.obj
	}
	for _, t := range cfg.Types {
		sc.Types = append(sc.Types, t.graphqlType())
	}
	s, err := gql.NewSchema(sc)
	if err != nil {
		return nil, err
	}
	return &Schema{s}, nil
}

// ExecuteParams carries the optional keyword arguments of
// GraphQL::Schema#execute: query variables, a per-request context, the operation
// to run when the document defines several, and a root value handed to the query
// root's resolvers.
type ExecuteParams struct {
	Variables     map[string]interface{}
	Context       context.Context
	OperationName string
	RootValue     map[string]interface{}
}

// Execute runs a query document against the schema and returns a graphql-ruby
// -shaped Result (data / errors). The optional ExecuteParams mirrors the keyword
// arguments of GraphQL::Schema#execute(query, variables:, context:,
// operation_name:); at most one may be supplied.
func (s *Schema) Execute(query string, params ...ExecuteParams) Result {
	var p ExecuteParams
	if len(params) > 0 {
		p = params[0]
	}
	gp := gql.Params{
		Schema:         s.schema,
		RequestString:  query,
		VariableValues: p.Variables,
		OperationName:  p.OperationName,
		RootObject:     p.RootValue,
		Context:        p.Context,
	}
	return newResult(gql.Do(gp))
}
