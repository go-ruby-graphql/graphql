package graphql

import (
	gql "github.com/graphql-go/graphql"
	"github.com/graphql-go/graphql/gqlerrors"
)

// ExecutionError mirrors GraphQL::ExecutionError. A resolver returns one (or any
// error) to signal a handled, client-facing failure: the field resolves to null
// and an entry is added to the response "errors" array with the field's path and
// source location, exactly as graphql-ruby does when a resolver raises a
// GraphQL::ExecutionError.
type ExecutionError struct {
	Message string
	// Extensions is attached to the formatted error under "extensions", mirroring
	// graphql-ruby's error extensions.
	Extensions map[string]interface{}
}

// NewExecutionError builds an ExecutionError with the given message.
func NewExecutionError(message string) *ExecutionError {
	return &ExecutionError{Message: message}
}

// Error implements the error interface.
func (e *ExecutionError) Error() string { return e.Message }

// Result mirrors the Hash returned by GraphQL::Schema#execute. It always exposes
// a "data" key for a query that parsed and validated (its value may be nil), and
// an "errors" key holding the collected errors when any occurred. A query that
// fails to parse or validate carries only "errors", with no "data" key — the
// same shape graphql-ruby produces.
type Result map[string]interface{}

// Data returns the "data" payload as a map, or nil when absent or null.
func (r Result) Data() map[string]interface{} {
	if d, ok := r["data"].(map[string]interface{}); ok {
		return d
	}
	return nil
}

// Errors returns the collected errors in graphql-ruby's hash shape: each entry
// has a "message" and, where available, "locations" and "path". It returns nil
// when the result carries no errors.
func (r Result) Errors() []map[string]interface{} {
	if errs, ok := r["errors"].([]map[string]interface{}); ok {
		return errs
	}
	return nil
}

// HasErrors reports whether the result carries any errors.
func (r Result) HasErrors() bool { return len(r.Errors()) > 0 }

// formatError renders a single graphql-go formatted error as a graphql-ruby
// error hash: {"message" => ..., "locations" => [{"line"=>,"column"=>}], "path"
// => [...]}. Locations and path are included only when present, matching
// graphql-ruby's serialization.
func formatError(e gqlerrors.FormattedError) map[string]interface{} {
	out := map[string]interface{}{"message": e.Message}
	if len(e.Locations) > 0 {
		locs := make([]map[string]interface{}, 0, len(e.Locations))
		for _, l := range e.Locations {
			locs = append(locs, map[string]interface{}{
				"line":   l.Line,
				"column": l.Column,
			})
		}
		out["locations"] = locs
	}
	if len(e.Path) > 0 {
		out["path"] = append([]interface{}{}, e.Path...)
	}
	if len(e.Extensions) > 0 {
		out["extensions"] = e.Extensions
	}
	return out
}

// hasPath reports whether any formatted error carries a response path. A path is
// present for errors raised during field execution but absent for parse and
// validation errors, which lets Execute reproduce graphql-ruby's rule of
// omitting the "data" key for purely static failures.
func hasPath(errs []gqlerrors.FormattedError) bool {
	for _, e := range errs {
		if len(e.Path) > 0 {
			return true
		}
	}
	return false
}

// newResult converts a graphql-go result into the graphql-ruby-shaped Result.
func newResult(gres *gql.Result) Result {
	res := Result{}
	// Include "data" for anything that reached execution: either data was
	// produced, there were no errors at all, or an execution (field) error
	// propagated. Omit it only for static parse/validation failures.
	if gres.Data != nil || len(gres.Errors) == 0 || hasPath(gres.Errors) {
		res["data"] = gres.Data
	}
	if len(gres.Errors) > 0 {
		errs := make([]map[string]interface{}, 0, len(gres.Errors))
		for _, e := range gres.Errors {
			errs = append(errs, formatError(e))
		}
		res["errors"] = errs
	}
	return res
}
