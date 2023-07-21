package q

import (
	"fmt"
	"regexp"
	"sync"
)

// Re creates a regexp matcher. It checks if the given field matches the given regexp.
// Note that this only supports fields of type string or []byte.
func Re(field string, re string) Matcher {
	regexpCache.RLock()
	if r, ok := regexpCache.m[re]; ok {
		regexpCache.RUnlock()
		return NewFieldMatcher(field, &regexpMatcher{r: r})
	}
	regexpCache.RUnlock()

	regexpCache.Lock()
	r, err := regexp.Compile(re)
	if err == nil {
		regexpCache.m[re] = r
	}
	regexpCache.Unlock()

	return NewFieldMatcher(field, &regexpMatcher{r: r, err: err})
}

var regexpCache = struct {
	sync.RWMutex
	m map[string]*regexp.Regexp
}{m: make(map[string]*regexp.Regexp)}

type regexpMatcher struct {
	r   *regexp.Regexp
	err error
}

func (r *regexpMatcher) MatchField(v interface{}) (bool, error) {
	if r.err != nil {
		return false, r.err
	}
	switch fieldValue := v.(type) {
	case string:
		return r.r.MatchString(fieldValue), nil
	case []byte:
		return r.r.Match(fieldValue), nil
	default:
		return false, fmt.Errorf("Only string and []byte supported for regexp matcher, got %T", fieldValue)
	}
}
