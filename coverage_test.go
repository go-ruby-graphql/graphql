package graphql

import (
	"testing"

	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
	"github.com/graphql-go/graphql/language/ast"
	"github.com/graphql-go/graphql/language/location"
)

func TestLiteralToValue(t *testing.T) {
	cases := []struct {
		node ast.Value
		want interface{}
	}{
		{&ast.IntValue{Value: "1"}, "1"},
		{&ast.FloatValue{Value: "1.5"}, "1.5"},
		{&ast.StringValue{Value: "hi"}, "hi"},
		{&ast.BooleanValue{Value: true}, true},
		{&ast.EnumValue{Value: "X"}, "X"},
		{&ast.ObjectValue{}, nil}, // default (unsupported literal) -> nil
	}
	for _, c := range cases {
		if got := literalToValue(c.node); got != c.want {
			t.Fatalf("literalToValue(%T) = %v, want %v", c.node, got, c.want)
		}
	}
}

func TestWrapResolveTypeBranches(t *testing.T) {
	if wrapResolveType(nil) != nil {
		t.Fatalf("wrapResolveType(nil) should be nil")
	}
	// A resolver that returns no concrete type -> the wrapper returns nil.
	fn := wrapResolveType(func(value interface{}) *ObjectType { return nil })
	if got := fn(gql.ResolveTypeParams{}); got != nil {
		t.Fatalf("expected nil concrete type, got %#v", got)
	}
	// And one that returns a concrete type -> its underlying object.
	ot := NewObjectType(ObjectTypeConfig{Name: "T", Fields: FieldMap{"a": {Type: String}}})
	fn = wrapResolveType(func(value interface{}) *ObjectType { return ot })
	if got := fn(gql.ResolveTypeParams{}); got != ot.obj {
		t.Fatalf("expected underlying object, got %#v", got)
	}
}

func TestWrapResolveNil(t *testing.T) {
	if wrapResolve(nil) != nil {
		t.Fatalf("wrapResolve(nil) should be nil")
	}
}

func TestBuildArgsNil(t *testing.T) {
	if buildArgs(nil) != nil {
		t.Fatalf("buildArgs(nil) should be nil")
	}
}

func TestScalarTypeConfigEmptyBranches(t *testing.T) {
	// Exercises the false branches of the Serialize / ParseValue guards; the
	// resulting scalar is intentionally not added to a schema.
	_ = NewScalarType(ScalarTypeConfig{Name: "Empty"})
	// Serialize present, ParseValue absent -> no ParseLiteral wired.
	_ = NewScalarType(ScalarTypeConfig{Name: "OnlySerialize", Serialize: func(v interface{}) interface{} { return v }})
}

func TestNewSchemaError(t *testing.T) {
	if _, err := NewSchema(SchemaConfig{}); err == nil {
		t.Fatalf("expected an error for a schema with no query root")
	}
}

func TestResultDataNonMap(t *testing.T) {
	// data present but null -> Data() returns nil via the type-assert miss.
	r := Result{"data": nil}
	if r.Data() != nil {
		t.Fatalf("Data() on null data should be nil")
	}
	// no errors key -> Errors() nil.
	if r.Errors() != nil {
		t.Fatalf("Errors() should be nil when absent")
	}
}

func TestFormatErrorFull(t *testing.T) {
	e := gqlerrors.FormattedError{
		Message:    "boom",
		Locations:  []location.SourceLocation{{Line: 2, Column: 5}},
		Path:       []interface{}{"a", 0, "b"},
		Extensions: map[string]interface{}{"code": "E_BOOM"},
	}
	out := formatError(e)
	if out["message"] != "boom" {
		t.Fatalf("message = %v", out["message"])
	}
	if out["locations"].([]map[string]interface{})[0]["line"] != 2 {
		t.Fatalf("locations = %#v", out["locations"])
	}
	if len(out["path"].([]interface{})) != 3 {
		t.Fatalf("path = %#v", out["path"])
	}
	if out["extensions"].(map[string]interface{})["code"] != "E_BOOM" {
		t.Fatalf("extensions = %#v", out["extensions"])
	}
}

func TestExecutionErrorAccessors(t *testing.T) {
	e := NewExecutionError("nope")
	if e.Error() != "nope" {
		t.Fatalf("Error() = %v", e.Error())
	}
}
