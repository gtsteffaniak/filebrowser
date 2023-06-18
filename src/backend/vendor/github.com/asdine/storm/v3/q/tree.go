// Package q contains a list of Matchers used to compare struct fields with values
package q

import (
	"go/token"
	"reflect"
)

// A Matcher is used to test against a record to see if it matches.
type Matcher interface {
	// Match is used to test the criteria against a structure.
	Match(interface{}) (bool, error)
}

// A ValueMatcher is used to test against a reflect.Value.
type ValueMatcher interface {
	// MatchValue tests if the given reflect.Value matches.
	// It is useful when the reflect.Value of an object already exists.
	MatchValue(*reflect.Value) (bool, error)
}

type cmp struct {
	value interface{}
	token token.Token
}

func (c *cmp) MatchField(v interface{}) (bool, error) {
	return compare(v, c.value, c.token), nil
}

type trueMatcher struct{}

func (*trueMatcher) Match(i interface{}) (bool, error) {
	return true, nil
}

func (*trueMatcher) MatchValue(v *reflect.Value) (bool, error) {
	return true, nil
}

type or struct {
	children []Matcher
}

func (c *or) Match(i interface{}) (bool, error) {
	v := reflect.Indirect(reflect.ValueOf(i))
	return c.MatchValue(&v)
}

func (c *or) MatchValue(v *reflect.Value) (bool, error) {
	for _, matcher := range c.children {
		if vm, ok := matcher.(ValueMatcher); ok {
			ok, err := vm.MatchValue(v)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
			continue
		}

		ok, err := matcher.Match(v.Interface())
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

type and struct {
	children []Matcher
}

func (c *and) Match(i interface{}) (bool, error) {
	v := reflect.Indirect(reflect.ValueOf(i))
	return c.MatchValue(&v)
}

func (c *and) MatchValue(v *reflect.Value) (bool, error) {
	for _, matcher := range c.children {
		if vm, ok := matcher.(ValueMatcher); ok {
			ok, err := vm.MatchValue(v)
			if err != nil {
				return false, err
			}
			if !ok {
				return false, nil
			}
			continue
		}

		ok, err := matcher.Match(v.Interface())
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	return true, nil
}

type strictEq struct {
	field string
	value interface{}
}

func (s *strictEq) MatchField(v interface{}) (bool, error) {
	return reflect.DeepEqual(v, s.value), nil
}

type in struct {
	list interface{}
}

func (i *in) MatchField(v interface{}) (bool, error) {
	ref := reflect.ValueOf(i.list)
	if ref.Kind() != reflect.Slice {
		return false, nil
	}

	c := cmp{
		token: token.EQL,
	}

	for i := 0; i < ref.Len(); i++ {
		c.value = ref.Index(i).Interface()
		ok, err := c.MatchField(v)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}

	return false, nil
}

type not struct {
	children []Matcher
}

func (n *not) Match(i interface{}) (bool, error) {
	v := reflect.Indirect(reflect.ValueOf(i))
	return n.MatchValue(&v)
}

func (n *not) MatchValue(v *reflect.Value) (bool, error) {
	var err error

	for _, matcher := range n.children {
		vm, ok := matcher.(ValueMatcher)
		if ok {
			ok, err = vm.MatchValue(v)
		} else {
			ok, err = matcher.Match(v.Interface())
		}
		if err != nil {
			return false, err
		}
		if ok {
			return false, nil
		}
	}

	return true, nil
}

// Eq matcher, checks if the given field is equal to the given value
func Eq(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &cmp{value: v, token: token.EQL})
}

// EqF matcher, checks if the given field is equal to the given field
func EqF(field1, field2 string) Matcher {
	return NewField2FieldMatcher(field1, field2, token.EQL)
}

// StrictEq matcher, checks if the given field is deeply equal to the given value
func StrictEq(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &strictEq{value: v})
}

// Gt matcher, checks if the given field is greater than the given value
func Gt(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &cmp{value: v, token: token.GTR})
}

// GtF matcher, checks if the given field is greater than the given field
func GtF(field1, field2 string) Matcher {
	return NewField2FieldMatcher(field1, field2, token.GTR)
}

// Gte matcher, checks if the given field is greater than or equal to the given value
func Gte(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &cmp{value: v, token: token.GEQ})
}

// GteF matcher, checks if the given field is greater than or equal to the given field
func GteF(field1, field2 string) Matcher {
	return NewField2FieldMatcher(field1, field2, token.GEQ)
}

// Lt matcher, checks if the given field is lesser than the given value
func Lt(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &cmp{value: v, token: token.LSS})
}

// LtF matcher, checks if the given field is lesser than the given field
func LtF(field1, field2 string) Matcher {
	return NewField2FieldMatcher(field1, field2, token.LSS)
}

// Lte matcher, checks if the given field is lesser than or equal to the given value
func Lte(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &cmp{value: v, token: token.LEQ})
}

// LteF matcher, checks if the given field is lesser than or equal to the given field
func LteF(field1, field2 string) Matcher {
	return NewField2FieldMatcher(field1, field2, token.LEQ)
}

// In matcher, checks if the given field matches one of the value of the given slice.
// v must be a slice.
func In(field string, v interface{}) Matcher {
	return NewFieldMatcher(field, &in{list: v})
}

// True matcher, always returns true
func True() Matcher { return &trueMatcher{} }

// Or matcher, checks if at least one of the given matchers matches the record
func Or(matchers ...Matcher) Matcher { return &or{children: matchers} }

// And matcher, checks if all of the given matchers matches the record
func And(matchers ...Matcher) Matcher { return &and{children: matchers} }

// Not matcher, checks if all of the given matchers return false
func Not(matchers ...Matcher) Matcher { return &not{children: matchers} }
