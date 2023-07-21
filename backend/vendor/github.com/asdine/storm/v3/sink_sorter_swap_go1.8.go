// +build go1.8

package storm

import "reflect"

func (s *sorter) Swap(i, j int) {
	// skip if we encountered an earlier error
	select {
	case <-s.done:
		return
	default:
	}

	if ssink, ok := s.sink.(sliceSink); ok {
		reflect.Swapper(ssink.slice().Interface())(i, j)
	} else {
		s.list[i], s.list[j] = s.list[j], s.list[i]
	}
}
