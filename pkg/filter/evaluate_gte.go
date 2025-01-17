package filter

import (
	"reflect"
)

type gte struct {
	ref float64
}

func newGTE(r interface{}) (Evaluator, error) {
	v, err := convertFloatValue(r)
	if err != nil {
		return nil, err
	}

	return &gte{ref: v}, nil
}

// Evaluate returns whether the actual value is greater than or equal the reference.
// It converts numerical values implicitly before comparison.
// Returns the lengths comparison for Array, Map, Slice or String.
// Returns false if the value is nil.
func (e *gte) Evaluate(v interface{}) bool {
	v = convertValue(v)

	if isNil(v) {
		return false
	}

	val := reflect.ValueOf(v)

	//nolint:exhaustive
	switch val.Kind() {
	case reflect.Float64:
		return val.Float() >= e.ref
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return val.Len() >= int(e.ref)
	}

	return false
}
