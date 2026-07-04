package graphql

import (
	"context"
	"reflect"
	"testing"
)

func mustData(t *testing.T, r Result) map[string]interface{} {
	t.Helper()
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %#v", r.Errors())
	}
	d := r.Data()
	if d == nil {
		t.Fatalf("expected data, got nil (result=%#v)", r)
	}
	return d
}

func TestNestedObjectsAndLists(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{ hero { name friends { name appearsIn } } }`)
	d := mustData(t, r)
	hero := d["hero"].(map[string]interface{})
	if hero["name"] != "Luke" {
		t.Fatalf("hero.name = %v", hero["name"])
	}
	friends := hero["friends"].([]interface{})
	if len(friends) != 1 || friends[0].(map[string]interface{})["name"] != "R2-D2" {
		t.Fatalf("friends = %#v", friends)
	}
	appearsIn := friends[0].(map[string]interface{})["appearsIn"].([]interface{})
	if !reflect.DeepEqual(appearsIn, []interface{}{"NEWHOPE", "EMPIRE", "JEDI"}) {
		t.Fatalf("appearsIn = %#v", appearsIn)
	}
}

func TestEnumArgumentAndDefault(t *testing.T) {
	s := buildFixtureSchema(t)
	// Default episode NEWHOPE -> Luke.
	if got := mustData(t, s.Execute(`{ hero { name } }`))["hero"].(map[string]interface{})["name"]; got != "Luke" {
		t.Fatalf("default hero = %v", got)
	}
	// Explicit enum JEDI -> R2-D2.
	if got := mustData(t, s.Execute(`{ hero(episode: JEDI) { name } }`))["hero"].(map[string]interface{})["name"]; got != "R2-D2" {
		t.Fatalf("JEDI hero = %v", got)
	}
}

func TestInterfaceInlineFragment(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{
		hero(episode: JEDI) {
			name
			... on Droid { primaryFunction }
			... on Human { homePlanet }
		}
	}`)
	d := mustData(t, r)
	hero := d["hero"].(map[string]interface{})
	if hero["primaryFunction"] != "Astromech" {
		t.Fatalf("droid primaryFunction = %v", hero["primaryFunction"])
	}
	if _, present := hero["homePlanet"]; present {
		t.Fatalf("homePlanet should be absent for a droid: %#v", hero)
	}
}

func TestUnionSearch(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{
		search(text: "r2") {
			... on Human { name }
			... on Droid { name primaryFunction }
		}
	}`)
	d := mustData(t, r)
	results := d["search"].([]interface{})
	if len(results) != 1 {
		t.Fatalf("search results = %#v", results)
	}
	first := results[0].(map[string]interface{})
	if first["name"] != "R2-D2" || first["primaryFunction"] != "Astromech" {
		t.Fatalf("union member = %#v", first)
	}
}

func TestVariablesFragmentsAndAliases(t *testing.T) {
	s := buildFixtureSchema(t)
	query := `
		query HeroAndHuman($id: ID!) {
			jedi: hero(episode: JEDI) { ...CharFields }
			luke: human(id: $id) { ...CharFields homePlanet }
		}
		fragment CharFields on Character { name }
	`
	r := s.Execute(query, ExecuteParams{
		Variables:     map[string]interface{}{"id": "1000"},
		OperationName: "HeroAndHuman",
	})
	d := mustData(t, r)
	if d["jedi"].(map[string]interface{})["name"] != "R2-D2" {
		t.Fatalf("jedi alias = %#v", d["jedi"])
	}
	luke := d["luke"].(map[string]interface{})
	if luke["name"] != "Luke" || luke["homePlanet"] != "Tatooine" {
		t.Fatalf("luke alias = %#v", luke)
	}
}

func TestIncludeSkipDirectives(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`query($withPlanet: Boolean!, $hide: Boolean!) {
		human(id: "1000") {
			name
			homePlanet @include(if: $withPlanet)
			id @skip(if: $hide)
		}
	}`, ExecuteParams{Variables: map[string]interface{}{"withPlanet": false, "hide": true}})
	human := mustData(t, r)["human"].(map[string]interface{})
	if _, ok := human["homePlanet"]; ok {
		t.Fatalf("homePlanet should be excluded: %#v", human)
	}
	if _, ok := human["id"]; ok {
		t.Fatalf("id should be skipped: %#v", human)
	}
	if human["name"] != "Luke" {
		t.Fatalf("name = %v", human["name"])
	}
}

func TestInputObjectAndDefault(t *testing.T) {
	s := buildFixtureSchema(t)
	// commentary omitted -> input-object default "n/a".
	r := s.Execute(`{ echo(review: { stars: 5 }) }`)
	if got := mustData(t, r)["echo"]; got != "n/a" {
		t.Fatalf("echo default = %v", got)
	}
	r = s.Execute(`{ echo(review: { stars: 5, commentary: "great" }) }`)
	if got := mustData(t, r)["echo"]; got != "great" {
		t.Fatalf("echo = %v", got)
	}
}

func TestCustomScalarLiteralAndVariable(t *testing.T) {
	s := buildFixtureSchema(t)
	// Literal path -> ParseLiteral -> ParseValue -> upper; Serialize -> upper.
	if got := mustData(t, s.Execute(`{ shout(text: "hi") }`))["shout"]; got != "HI" {
		t.Fatalf("shout literal = %v", got)
	}
	// Variable path -> ParseValue.
	r := s.Execute(`query($t: UpperString!) { shout(text: $t) }`,
		ExecuteParams{Variables: map[string]interface{}{"t": "yo"}})
	if got := mustData(t, r)["shout"]; got != "YO" {
		t.Fatalf("shout var = %v", got)
	}
}

func TestMutation(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`mutation { addReview(review: { stars: 4, commentary: "ok" }) }`)
	if got := mustData(t, r)["addReview"]; got != 4 {
		t.Fatalf("addReview = %v", got)
	}
}

func TestFieldErrorPathAndLocation(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{ boom }`)
	// data present with the field nulled.
	if r.Data() == nil {
		t.Fatalf("expected data with nulled field, got %#v", r)
	}
	if v, ok := r.Data()["boom"]; !ok || v != nil {
		t.Fatalf("boom should be null: %#v", r.Data())
	}
	errs := r.Errors()
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %#v", errs)
	}
	e := errs[0]
	if e["message"] != "kaboom" {
		t.Fatalf("message = %v", e["message"])
	}
	path := e["path"].([]interface{})
	if len(path) != 1 || path[0] != "boom" {
		t.Fatalf("path = %#v", path)
	}
	locs := e["locations"].([]map[string]interface{})
	if len(locs) != 1 || locs[0]["line"].(int) < 1 || locs[0]["column"].(int) < 1 {
		t.Fatalf("locations = %#v", locs)
	}
}

func TestNonNullErrorPropagates(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{ nonNullBoom }`)
	// A non-null field error propagates to the root: data is null but present.
	if !r.HasErrors() {
		t.Fatalf("expected errors")
	}
	d, present := r["data"]
	if !present {
		t.Fatalf("data key should be present (null) for a propagated execution error: %#v", r)
	}
	if d != nil {
		t.Fatalf("data should be null, got %#v", d)
	}
	if r.Errors()[0]["message"] != "hard kaboom" {
		t.Fatalf("message = %v", r.Errors()[0]["message"])
	}
}

func TestParseError(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{ hero { name `) // unterminated
	if _, present := r["data"]; present {
		t.Fatalf("parse error must not carry a data key: %#v", r)
	}
	if !r.HasErrors() {
		t.Fatalf("expected parse errors")
	}
	if _, hasPath := r.Errors()[0]["path"]; hasPath {
		t.Fatalf("parse error should have no path: %#v", r.Errors()[0])
	}
}

func TestValidationError(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{ hero { noSuchField } }`)
	if _, present := r["data"]; present {
		t.Fatalf("validation error must not carry a data key: %#v", r)
	}
	if !r.HasErrors() {
		t.Fatalf("expected validation errors")
	}
	// Validation errors carry a location but no path.
	if _, ok := r.Errors()[0]["locations"]; !ok {
		t.Fatalf("expected a location on the validation error: %#v", r.Errors()[0])
	}
}

func TestContextIsThreaded(t *testing.T) {
	s := buildFixtureSchema(t)
	type ctxKey string
	ctx := context.WithValue(context.Background(), ctxKey("k"), "v")
	r := s.Execute(`{ hero { name } }`, ExecuteParams{Context: ctx})
	mustData(t, r) // resolvers run; context path exercised
}

func TestIntrospection(t *testing.T) {
	s := buildFixtureSchema(t)
	r := s.Execute(`{
		__schema { queryType { name } mutationType { name } subscriptionType { name } }
		droidType: __type(name: "Droid") { name kind fields { name } interfaces { name } }
	}`)
	d := mustData(t, r)
	sch := d["__schema"].(map[string]interface{})
	if sch["queryType"].(map[string]interface{})["name"] != "Query" {
		t.Fatalf("queryType = %#v", sch["queryType"])
	}
	if sch["mutationType"].(map[string]interface{})["name"] != "Mutation" {
		t.Fatalf("mutationType = %#v", sch["mutationType"])
	}
	if sch["subscriptionType"].(map[string]interface{})["name"] != "Subscription" {
		t.Fatalf("subscriptionType = %#v", sch["subscriptionType"])
	}
	dt := d["droidType"].(map[string]interface{})
	if dt["name"] != "Droid" || dt["kind"] != "OBJECT" {
		t.Fatalf("droidType = %#v", dt)
	}
	if len(dt["interfaces"].([]interface{})) != 1 {
		t.Fatalf("droid interfaces = %#v", dt["interfaces"])
	}
}
