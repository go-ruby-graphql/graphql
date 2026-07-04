package graphql

import (
	"strings"
	"testing"
)

// The fixture is a small, self-contained Star-Wars-style schema exercising the
// full surface: nested objects, lists, an enum, an interface with two
// implementers, a union, an input object, a custom scalar, arguments with
// defaults, and resolvers that succeed and that fail with an ExecutionError.

type character struct {
	id          string
	name        string
	friendIDs   []string
	appearsIn   []string
	kind        string // "Human" or "Droid"
	homePlanet  string
	primaryFunc string
}

var characters = map[string]*character{
	"1000": {id: "1000", name: "Luke", friendIDs: []string{"2001"}, appearsIn: []string{"NEWHOPE", "EMPIRE"}, kind: "Human", homePlanet: "Tatooine"},
	"2001": {id: "2001", name: "R2-D2", friendIDs: []string{"1000"}, appearsIn: []string{"NEWHOPE", "EMPIRE", "JEDI"}, kind: "Droid", primaryFunc: "Astromech"},
}

// buildFixtureSchema assembles the fixture schema and returns it, failing the
// test if construction is invalid.
func buildFixtureSchema(t *testing.T) *Schema {
	t.Helper()

	episode := NewEnumType(EnumTypeConfig{
		Name: "Episode",
		Values: EnumValueMap{
			"NEWHOPE": {Value: "NEWHOPE"},
			"EMPIRE":  {Value: "EMPIRE"},
			"JEDI":    {Description: "Return of the Jedi"}, // nil Value -> uses name
		},
	})

	// A custom scalar: upper-cases on the way in and out.
	upper := NewScalarType(ScalarTypeConfig{
		Name:        "UpperString",
		Description: "A string coerced to upper case",
		Serialize: func(v interface{}) interface{} {
			s, _ := v.(string)
			return strings.ToUpper(s)
		},
		ParseValue: func(v interface{}) interface{} {
			s, _ := v.(string)
			return strings.ToUpper(s)
		},
	})

	var characterType *InterfaceType
	var humanType, droidType *ObjectType

	resolveCharKind := func(value interface{}) *ObjectType {
		if c, ok := value.(*character); ok && c.kind == "Droid" {
			return droidType
		}
		return humanType
	}

	characterType = NewInterfaceType(InterfaceTypeConfig{
		Name:        "Character",
		Description: "A character in the saga",
		Fields: FieldMap{
			"id":        {Type: NonNullType(ID)},
			"name":      {Type: String},
			"appearsIn": {Type: ListType(episode)},
			"friends":   {Type: nil}, // set below (self-reference)
		},
		ResolveType: resolveCharKind,
	})

	nameResolver := func(p ResolveParams) (interface{}, error) {
		return p.Source.(*character).name, nil
	}
	idResolver := func(p ResolveParams) (interface{}, error) {
		return p.Source.(*character).id, nil
	}
	appearsResolver := func(p ResolveParams) (interface{}, error) {
		out := make([]interface{}, 0)
		for _, e := range p.Source.(*character).appearsIn {
			out = append(out, e)
		}
		return out, nil
	}
	friendsResolver := func(p ResolveParams) (interface{}, error) {
		out := make([]interface{}, 0)
		for _, id := range p.Source.(*character).friendIDs {
			out = append(out, characters[id])
		}
		return out, nil
	}

	humanType = NewObjectType(ObjectTypeConfig{
		Name:       "Human",
		Interfaces: []*InterfaceType{characterType},
		Fields: FieldMap{
			"id":        {Type: NonNullType(ID), Resolve: idResolver},
			"name":      {Type: String, Resolve: nameResolver},
			"appearsIn": {Type: ListType(episode), Resolve: appearsResolver},
			"friends":   {Type: nil}, // set below
			"homePlanet": {Type: String, Resolve: func(p ResolveParams) (interface{}, error) {
				return p.Source.(*character).homePlanet, nil
			}},
		},
	})
	droidType = NewObjectType(ObjectTypeConfig{
		Name:       "Droid",
		Interfaces: []*InterfaceType{characterType},
		Fields: FieldMap{
			"id":        {Type: NonNullType(ID), Resolve: idResolver},
			"name":      {Type: String, Resolve: nameResolver},
			"appearsIn": {Type: ListType(episode), Resolve: appearsResolver},
			"friends":   {Type: nil}, // set below
			"primaryFunction": {Type: String, Resolve: func(p ResolveParams) (interface{}, error) {
				return p.Source.(*character).primaryFunc, nil
			}},
		},
	})

	// Wire the self-referential "friends" field now that the object types exist.
	friendsField := &Field{Type: ListType(characterType), Resolve: friendsResolver}
	characterType.iface.AddFieldConfig("friends", buildField(friendsField))
	humanType.obj.AddFieldConfig("friends", buildField(friendsField))
	droidType.obj.AddFieldConfig("friends", buildField(friendsField))

	searchResult := NewUnionType(UnionTypeConfig{
		Name:        "SearchResult",
		Types:       []*ObjectType{humanType, droidType},
		ResolveType: resolveCharKind,
	})

	reviewInput := NewInputObjectType(InputObjectTypeConfig{
		Name: "ReviewInput",
		Fields: InputFieldMap{
			"stars":      {Type: NonNullType(Int)},
			"commentary": {Type: String, DefaultValue: "n/a"},
		},
	})

	charByID := func(p ResolveParams) (interface{}, error) {
		return characters[p.Args["id"].(string)], nil
	}

	queryType := NewObjectType(ObjectTypeConfig{
		Name: "Query",
		Fields: FieldMap{
			"hero": {
				Type: characterType,
				Args: ArgumentMap{"episode": {Type: episode, DefaultValue: "NEWHOPE"}},
				Resolve: func(p ResolveParams) (interface{}, error) {
					if p.Args["episode"] == "JEDI" {
						return characters["2001"], nil
					}
					return characters["1000"], nil
				},
			},
			"human": {Type: humanType, Args: ArgumentMap{"id": {Type: NonNullType(ID)}}, Resolve: charByID},
			"droid": {Type: droidType, Args: ArgumentMap{"id": {Type: NonNullType(ID)}}, Resolve: charByID},
			"search": {
				Type: ListType(searchResult),
				Args: ArgumentMap{"text": {Type: NonNullType(String)}},
				Resolve: func(p ResolveParams) (interface{}, error) {
					text := strings.ToLower(p.Args["text"].(string))
					out := make([]interface{}, 0)
					for _, c := range []string{"1000", "2001"} {
						if strings.Contains(strings.ToLower(characters[c].name), text) {
							out = append(out, characters[c])
						}
					}
					return out, nil
				},
			},
			"echo": {
				Type: String,
				Args: ArgumentMap{"review": {Type: reviewInput}},
				Resolve: func(p ResolveParams) (interface{}, error) {
					r := p.Args["review"].(map[string]interface{})
					return r["commentary"], nil
				},
			},
			"shout": {
				Type:    upper,
				Args:    ArgumentMap{"text": {Type: NonNullType(upper)}},
				Resolve: func(p ResolveParams) (interface{}, error) { return p.Args["text"], nil },
			},
			"boom": {
				Type:    String,
				Resolve: func(p ResolveParams) (interface{}, error) { return nil, NewExecutionError("kaboom") },
			},
			"nonNullBoom": {
				Type:    NonNullType(String),
				Resolve: func(p ResolveParams) (interface{}, error) { return nil, NewExecutionError("hard kaboom") },
			},
		},
	})

	mutationType := NewObjectType(ObjectTypeConfig{
		Name: "Mutation",
		Fields: FieldMap{
			"addReview": {
				Type: Int,
				Args: ArgumentMap{"review": {Type: NonNullType(reviewInput)}},
				Resolve: func(p ResolveParams) (interface{}, error) {
					r := p.Args["review"].(map[string]interface{})
					return r["stars"], nil
				},
			},
		},
	})

	subscriptionType := NewObjectType(ObjectTypeConfig{
		Name: "Subscription",
		Fields: FieldMap{
			"reviewAdded": {Type: Int, Resolve: func(p ResolveParams) (interface{}, error) { return 5, nil }},
		},
	})

	schema, err := NewSchema(SchemaConfig{
		Query:        queryType,
		Mutation:     mutationType,
		Subscription: subscriptionType,
		Types:        []Type{humanType, droidType},
	})
	if err != nil {
		t.Fatalf("building fixture schema: %v", err)
	}
	return schema
}
