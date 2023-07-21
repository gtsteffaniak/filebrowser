// +build !go1.8

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
		x, y := ssink.slice().Index(i).Interface(), ssink.slice().Index(j).Interface()
		ssink.slice().Index(i).Set(reflect.ValueOf(y))
		ssink.slice().Index(j).Set(reflect.ValueOf(x))
	} else {
		s.list[i], s.list[j] = s.list[j], s.list[i]
	}
}
