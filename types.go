package graphql

import (
	"context"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/language/ast"
)

// Type is a member of the GraphQL type system. It mirrors the abstract
// GraphQL::Schema::Member hierarchy of graphql-ruby: every concrete type
// (scalars, objects, enums, interfaces, unions, input objects, and the List /
// NonNull wrappers) satisfies it, so it may be used as a field's return type or
// an argument's input type.
//
// The underlying graphql-go type is intentionally hidden behind an unexported
// accessor so that callers program against this package's Ruby-flavoured
// surface rather than graphql-go directly.
type Type interface {
	// graphqlType returns the wrapped graphql-go type.
	graphqlType() gql.Type
}

// ScalarType mirrors a GraphQL scalar (GraphQL::ScalarType). The five built-in
// scalars are exposed as the package-level values Int, Float, String, Boolean
// and ID; custom scalars are created with NewScalarType.
type ScalarType struct{ s *gql.Scalar }

func (t *ScalarType) graphqlType() gql.Type { return t.s }

// The five built-in scalars, mirroring GraphQL::Types::Int, ::Float, ::String,
// ::Boolean and ::ID.
var (
	Int     = &ScalarType{gql.Int}
	Float   = &ScalarType{gql.Float}
	String  = &ScalarType{gql.String}
	Boolean = &ScalarType{gql.Boolean}
	ID      = &ScalarType{gql.ID}
)

// ScalarTypeConfig configures a custom scalar. It mirrors a graphql-ruby custom
// scalar's coerce_result (Serialize) and coerce_input (ParseValue) hooks. The
// literal form used inside a query document is coerced with the same ParseValue
// function after the raw literal node is reduced to a Go value.
type ScalarTypeConfig struct {
	Name        string
	Description string
	// Serialize coerces an internal value to the value sent to the client
	// (graphql-ruby's coerce_result). Required.
	Serialize func(value interface{}) interface{}
	// ParseValue coerces a client-supplied value (a variable) to the internal
	// representation (graphql-ruby's coerce_input). Optional; required if the
	// scalar is used as an input type.
	ParseValue func(value interface{}) interface{}
}

// NewScalarType creates a custom scalar type.
func NewScalarType(cfg ScalarTypeConfig) *ScalarType {
	sc := gql.ScalarConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
	}
	if cfg.Serialize != nil {
		sc.Serialize = gql.SerializeFn(cfg.Serialize)
	}
	if cfg.ParseValue != nil {
		sc.ParseValue = gql.ParseValueFn(cfg.ParseValue)
		sc.ParseLiteral = func(valueAST ast.Value) interface{} {
			return cfg.ParseValue(literalToValue(valueAST))
		}
	}
	return &ScalarType{gql.NewScalar(sc)}
}

// literalToValue reduces a scalar literal node from a query document to a plain
// Go value so a custom scalar's ParseValue can coerce it, keeping graphql-go's
// ast package out of this package's public surface.
func literalToValue(valueAST ast.Value) interface{} {
	switch v := valueAST.(type) {
	case *ast.IntValue:
		return v.Value
	case *ast.FloatValue:
		return v.Value
	case *ast.StringValue:
		return v.Value
	case *ast.BooleanValue:
		return v.Value
	case *ast.EnumValue:
		return v.Value
	default:
		return nil
	}
}

// ObjectType mirrors GraphQL::ObjectType: a composite type with named,
// resolvable fields. Query, Mutation and Subscription roots are ordinary object
// types.
type ObjectType struct{ obj *gql.Object }

func (t *ObjectType) graphqlType() gql.Type { return t.obj }

// FieldMap maps field names to their definitions.
type FieldMap map[string]*Field

// Field mirrors GraphQL::Field: a named, typed, resolvable member of an object
// or interface type.
type Field struct {
	Type              Type
	Description       string
	Args              ArgumentMap
	Resolve           ResolveFn
	DeprecationReason string
}

// ArgumentMap maps argument names to their definitions.
type ArgumentMap map[string]*Argument

// Argument mirrors GraphQL::Argument: a named input to a field.
type Argument struct {
	Type         Type
	DefaultValue interface{}
	Description  string
}

// ResolveParams is passed to a field's resolver. It mirrors the object handed
// to a graphql-ruby resolver method: the parent object (Source), the coerced
// arguments (Args) and the per-request context.
type ResolveParams struct {
	Source  interface{}
	Args    map[string]interface{}
	Context context.Context
	info    gql.ResolveInfo
}

// ResolveFn is the field-resolver seam: given the resolve parameters it returns
// the field's value or an error. Returning an *ExecutionError (or any error)
// collects a field error at the current path, matching graphql-ruby.
type ResolveFn func(p ResolveParams) (interface{}, error)

func wrapResolve(fn ResolveFn) gql.FieldResolveFn {
	if fn == nil {
		return nil
	}
	return func(p gql.ResolveParams) (interface{}, error) {
		return fn(ResolveParams{
			Source:  p.Source,
			Args:    p.Args,
			Context: p.Context,
			info:    p.Info,
		})
	}
}

func buildArgs(args ArgumentMap) gql.FieldConfigArgument {
	if args == nil {
		return nil
	}
	out := gql.FieldConfigArgument{}
	for name, a := range args {
		out[name] = &gql.ArgumentConfig{
			Type:         a.Type.graphqlType(),
			DefaultValue: a.DefaultValue,
			Description:  a.Description,
		}
	}
	return out
}

// buildField converts one wrapper Field into a graphql-go field definition.
func buildField(f *Field) *gql.Field {
	return &gql.Field{
		Type:              f.Type.graphqlType(),
		Description:       f.Description,
		Args:              buildArgs(f.Args),
		Resolve:           wrapResolve(f.Resolve),
		DeprecationReason: f.DeprecationReason,
	}
}

func buildFields(fields FieldMap) gql.Fields {
	out := gql.Fields{}
	for name, f := range fields {
		if f.Type == nil {
			// A nil-typed placeholder (used to break a self-referential type
			// cycle); the real field is wired in after the types exist.
			continue
		}
		out[name] = buildField(f)
	}
	return out
}

// ObjectTypeConfig configures an object type.
type ObjectTypeConfig struct {
	Name        string
	Description string
	Fields      FieldMap
	// Interfaces lists the interface types this object implements.
	Interfaces []*InterfaceType
}

// NewObjectType creates an object type (GraphQL::ObjectType).
func NewObjectType(cfg ObjectTypeConfig) *ObjectType {
	var ifaces []*gql.Interface
	for _, i := range cfg.Interfaces {
		ifaces = append(ifaces, i.iface)
	}
	return &ObjectType{gql.NewObject(gql.ObjectConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Fields:      buildFields(cfg.Fields),
		Interfaces:  ifaces,
	})}
}

// InterfaceType mirrors GraphQL::InterfaceType: an abstract type defining a set
// of fields that implementing object types must provide.
type InterfaceType struct{ iface *gql.Interface }

func (t *InterfaceType) graphqlType() gql.Type { return t.iface }

// InterfaceTypeConfig configures an interface type.
type InterfaceTypeConfig struct {
	Name        string
	Description string
	Fields      FieldMap
	// ResolveType maps a runtime value to the concrete object type that
	// represents it, mirroring graphql-ruby's resolve_type hook.
	ResolveType func(value interface{}) *ObjectType
}

// NewInterfaceType creates an interface type (GraphQL::InterfaceType).
func NewInterfaceType(cfg InterfaceTypeConfig) *InterfaceType {
	return &InterfaceType{gql.NewInterface(gql.InterfaceConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Fields:      buildFields(cfg.Fields),
		ResolveType: wrapResolveType(cfg.ResolveType),
	})}
}

func wrapResolveType(fn func(value interface{}) *ObjectType) gql.ResolveTypeFn {
	if fn == nil {
		return nil
	}
	return func(p gql.ResolveTypeParams) *gql.Object {
		if ot := fn(p.Value); ot != nil {
			return ot.obj
		}
		return nil
	}
}

// UnionType mirrors GraphQL::UnionType: a type that is exactly one of a fixed
// set of object types.
type UnionType struct{ u *gql.Union }

func (t *UnionType) graphqlType() gql.Type { return t.u }

// UnionTypeConfig configures a union type.
type UnionTypeConfig struct {
	Name        string
	Description string
	Types       []*ObjectType
	// ResolveType maps a runtime value to its concrete member type.
	ResolveType func(value interface{}) *ObjectType
}

// NewUnionType creates a union type (GraphQL::UnionType).
func NewUnionType(cfg UnionTypeConfig) *UnionType {
	var types []*gql.Object
	for _, t := range cfg.Types {
		types = append(types, t.obj)
	}
	return &UnionType{gql.NewUnion(gql.UnionConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Types:       types,
		ResolveType: wrapResolveType(cfg.ResolveType),
	})}
}

// EnumType mirrors GraphQL::EnumType.
type EnumType struct{ e *gql.Enum }

func (t *EnumType) graphqlType() gql.Type { return t.e }

// EnumValue mirrors a single graphql-ruby enum value definition.
type EnumValue struct {
	// Value is the internal Go value this enum member coerces to; when nil the
	// member's name is used, mirroring graphql-ruby.
	Value             interface{}
	Description       string
	DeprecationReason string
}

// EnumValueMap maps enum member names to their definitions.
type EnumValueMap map[string]*EnumValue

// EnumTypeConfig configures an enum type.
type EnumTypeConfig struct {
	Name        string
	Description string
	Values      EnumValueMap
}

// NewEnumType creates an enum type (GraphQL::EnumType).
func NewEnumType(cfg EnumTypeConfig) *EnumType {
	values := gql.EnumValueConfigMap{}
	for name, v := range cfg.Values {
		values[name] = &gql.EnumValueConfig{
			Value:             v.Value,
			Description:       v.Description,
			DeprecationReason: v.DeprecationReason,
		}
	}
	return &EnumType{gql.NewEnum(gql.EnumConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Values:      values,
	})}
}

// InputObjectType mirrors GraphQL::InputObjectType: a composite input type used
// for structured arguments.
type InputObjectType struct{ io *gql.InputObject }

func (t *InputObjectType) graphqlType() gql.Type { return t.io }

// InputField mirrors a single field of an input object.
type InputField struct {
	Type         Type
	DefaultValue interface{}
	Description  string
}

// InputFieldMap maps input-field names to their definitions.
type InputFieldMap map[string]*InputField

// InputObjectTypeConfig configures an input object type.
type InputObjectTypeConfig struct {
	Name        string
	Description string
	Fields      InputFieldMap
}

// NewInputObjectType creates an input object type (GraphQL::InputObjectType).
func NewInputObjectType(cfg InputObjectTypeConfig) *InputObjectType {
	fields := gql.InputObjectConfigFieldMap{}
	for name, f := range cfg.Fields {
		fields[name] = &gql.InputObjectFieldConfig{
			Type:         f.Type.graphqlType(),
			DefaultValue: f.DefaultValue,
			Description:  f.Description,
		}
	}
	return &InputObjectType{gql.NewInputObject(gql.InputObjectConfig{
		Name:        cfg.Name,
		Description: cfg.Description,
		Fields:      fields,
	})}
}

// wrappedType carries a List or NonNull wrapper around another type.
type wrappedType struct{ t gql.Type }

func (w wrappedType) graphqlType() gql.Type { return w.t }

// ListType wraps a type as a GraphQL list (GraphQL::ListType, the Ruby [T]).
func ListType(of Type) Type {
	return wrappedType{gql.NewList(of.graphqlType())}
}

// NonNullType wraps a type as non-null (GraphQL::NonNullType, the Ruby T!).
func NonNullType(of Type) Type {
	return wrappedType{gql.NewNonNull(of.graphqlType())}
}
