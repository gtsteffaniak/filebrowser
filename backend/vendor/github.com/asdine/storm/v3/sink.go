package storm

import (
	"reflect"
	"sort"
	"time"
	"github.com/asdine/storm/v3/index"
	"github.com/asdine/storm/v3/q"
	bolt "go.etcd.io/bbolt"
)

type item struct {
	value  *reflect.Value
	bucket *bolt.Bucket
	k      []byte
	v      []byte
}

func newSorter(n Node, snk sink) *sorter {
	return &sorter{
		node:  n,
		sink:  snk,
		skip:  0,
		limit: -1,
		list:  make([]*item, 0),
		err:   make(chan error),
		done:  make(chan struct{}),
	}
}

type sorter struct {
	node    Node
	sink    sink
	list    []*item
	skip    int
	limit   int
	orderBy []string
	reverse bool
	err     chan error
	done    chan struct{}
}

func (s *sorter) filter(tree q.Matcher, bucket *bolt.Bucket, k, v []byte) (bool, error) {
	itm := &item{
		bucket: bucket,
		k:      k,
		v:      v,
	}
	rsink, ok := s.sink.(reflectSink)
	if !ok {
		return s.add(itm)
	}

	newElem := rsink.elem()
	if err := s.node.Codec().Unmarshal(v, newElem.Interface()); err != nil {
		return false, err
	}
	itm.value = &newElem

	if tree != nil {
		ok, err := tree.Match(newElem.Interface())
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}

	if len(s.orderBy) == 0 {
		return s.add(itm)
	}

	if _, ok := s.sink.(sliceSink); ok {
		// add directly to sink, we'll apply skip/limits after sorting
		return false, s.sink.add(itm)
	}

	s.list = append(s.list, itm)

	return false, nil
}

func (s *sorter) add(itm *item) (stop bool, err error) {
	if s.limit == 0 {
		return true, nil
	}

	if s.skip > 0 {
		s.skip--
		return false, nil
	}

	if s.limit > 0 {
		s.limit--
	}

	err = s.sink.add(itm)

	return s.limit == 0, err
}

func (s *sorter) compareValue(left reflect.Value, right reflect.Value) int {
	if !left.IsValid() || !right.IsValid() {
		if left.IsValid() {
			return 1
		}
		return -1
	}

	switch left.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		l, r := left.Int(), right.Int()
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		l, r := left.Uint(), right.Uint()
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	case reflect.Float32, reflect.Float64:
		l, r := left.Float(), right.Float()
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	case reflect.String:
		l, r := left.String(), right.String()
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	case reflect.Struct:
		if lt, lok := left.Interface().(time.Time); lok {
			if rt, rok := right.Interface().(time.Time); rok {
				if lok && rok {
					if lt.Before(rt) {
						return -1
					} else {
						return 1
					}
				}
			}
		}
	default:
		rawLeft, err := toBytes(left.Interface(), s.node.Codec())
		if err != nil {
			return -1
		}
		rawRight, err := toBytes(right.Interface(), s.node.Codec())
		if err != nil {
			return 1
		}

		l, r := string(rawLeft), string(rawRight)
		if l < r {
			return -1
		}
		if l > r {
			return 1
		}
	}

	return 0
}

func (s *sorter) less(leftElem reflect.Value, rightElem reflect.Value) bool {
	for _, orderBy := range s.orderBy {
		leftField := reflect.Indirect(leftElem).FieldByName(orderBy)
		if !leftField.IsValid() {
			s.err <- ErrNotFound
			return false
		}
		rightField := reflect.Indirect(rightElem).FieldByName(orderBy)
		if !rightField.IsValid() {
			s.err <- ErrNotFound
			return false
		}

		direction := 1
		if s.reverse {
			direction = -1
		}

		switch s.compareValue(leftField, rightField) * direction {
		case -1:
			return true
		case 1:
			return false
		default:
			continue
		}
	}

	return false
}

func (s *sorter) flush() error {
	if len(s.orderBy) == 0 {
		return s.sink.flush()
	}

	go func() {
		sort.Sort(s)
		close(s.err)
	}()
	err := <-s.err
	close(s.done)

	if err != nil {
		return err
	}

	if ssink, ok := s.sink.(sliceSink); ok {
		if !ssink.slice().IsValid() {
			return s.sink.flush()
		}
		if s.skip >= ssink.slice().Len() {
			ssink.reset()
			return s.sink.flush()
		}
		leftBound := s.skip
		if leftBound < 0 {
			leftBound = 0
		}
		limit := s.limit
		if s.limit < 0 {
			limit = 0
		}

		rightBound := leftBound + limit
		if rightBound > ssink.slice().Len() || rightBound == leftBound {
			rightBound = ssink.slice().Len()
		}
		ssink.setSlice(ssink.slice().Slice(leftBound, rightBound))
		return s.sink.flush()
	}

	for _, itm := range s.list {
		if itm == nil {
			break
		}
		stop, err := s.add(itm)
		if err != nil {
			return err
		}
		if stop {
			break
		}
	}

	return s.sink.flush()
}

func (s *sorter) Len() int {
	// skip if we encountered an earlier error
	select {
	case <-s.done:
		return 0
	default:
	}
	if ssink, ok := s.sink.(sliceSink); ok {
		return ssink.slice().Len()
	}
	return len(s.list)

}

func (s *sorter) Less(i, j int) bool {
	// skip if we encountered an earlier error
	select {
	case <-s.done:
		return false
	default:
	}

	if ssink, ok := s.sink.(sliceSink); ok {
		return s.less(ssink.slice().Index(i), ssink.slice().Index(j))
	}
	return s.less(*s.list[i].value, *s.list[j].value)
}

type sink interface {
	bucketName() string
	flush() error
	add(*item) error
	readOnly() bool
}

type reflectSink interface {
	elem() reflect.Value
}

type sliceSink interface {
	slice() reflect.Value
	setSlice(reflect.Value)
	reset()
}

func newListSink(node Node, to interface{}) (*listSink, error) {
	ref := reflect.ValueOf(to)

	if ref.Kind() != reflect.Ptr || reflect.Indirect(ref).Kind() != reflect.Slice {
		return nil, ErrSlicePtrNeeded
	}

	sliceType := reflect.Indirect(ref).Type()
	elemType := sliceType.Elem()

	if elemType.Kind() == reflect.Ptr {
		elemType = elemType.Elem()
	}

	if elemType.Name() == "" {
		return nil, ErrNoName
	}

	return &listSink{
		node:     node,
		ref:      ref,
		isPtr:    sliceType.Elem().Kind() == reflect.Ptr,
		elemType: elemType,
		name:     elemType.Name(),
		results:  reflect.MakeSlice(reflect.Indirect(ref).Type(), 0, 0),
	}, nil
}

type listSink struct {
	node     Node
	ref      reflect.Value
	results  reflect.Value
	elemType reflect.Type
	name     string
	isPtr    bool
	idx      int
}

func (l *listSink) slice() reflect.Value {
	return l.results
}

func (l *listSink) setSlice(s reflect.Value) {
	l.results = s
}

func (l *listSink) reset() {
	l.results = reflect.MakeSlice(reflect.Indirect(l.ref).Type(), 0, 0)
}

func (l *listSink) elem() reflect.Value {
	if l.results.IsValid() && l.idx < l.results.Len() {
		return l.results.Index(l.idx).Addr()
	}
	return reflect.New(l.elemType)
}

func (l *listSink) bucketName() string {
	return l.name
}

func (l *listSink) add(i *item) error {
	if l.idx == l.results.Len() {
		if l.isPtr {
			l.results = reflect.Append(l.results, *i.value)
		} else {
			l.results = reflect.Append(l.results, reflect.Indirect(*i.value))
		}
	}

	l.idx++

	return nil
}

func (l *listSink) flush() error {
	if l.results.IsValid() && l.results.Len() > 0 {
		reflect.Indirect(l.ref).Set(l.results)
		return nil
	}

	return ErrNotFound
}

func (l *listSink) readOnly() bool {
	return true
}

func newFirstSink(node Node, to interface{}) (*firstSink, error) {
	ref := reflect.ValueOf(to)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return nil, ErrStructPtrNeeded
	}

	return &firstSink{
		node: node,
		ref:  ref,
	}, nil
}

type firstSink struct {
	node  Node
	ref   reflect.Value
	found bool
}

func (f *firstSink) elem() reflect.Value {
	return reflect.New(reflect.Indirect(f.ref).Type())
}

func (f *firstSink) bucketName() string {
	return reflect.Indirect(f.ref).Type().Name()
}

func (f *firstSink) add(i *item) error {
	reflect.Indirect(f.ref).Set(i.value.Elem())
	f.found = true
	return nil
}

func (f *firstSink) flush() error {
	if !f.found {
		return ErrNotFound
	}

	return nil
}

func (f *firstSink) readOnly() bool {
	return true
}

func newDeleteSink(node Node, kind interface{}) (*deleteSink, error) {
	ref := reflect.ValueOf(kind)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return nil, ErrStructPtrNeeded
	}

	return &deleteSink{
		node: node,
		ref:  ref,
	}, nil
}

type deleteSink struct {
	node    Node
	ref     reflect.Value
	removed int
}

func (d *deleteSink) elem() reflect.Value {
	return reflect.New(reflect.Indirect(d.ref).Type())
}

func (d *deleteSink) bucketName() string {
	return reflect.Indirect(d.ref).Type().Name()
}

func (d *deleteSink) add(i *item) error {
	info, err := extract(&d.ref)
	if err != nil {
		return err
	}

	for fieldName, fieldCfg := range info.Fields {
		if fieldCfg.Index == "" {
			continue
		}
		idx, err := getIndex(i.bucket, fieldCfg.Index, fieldName)
		if err != nil {
			return err
		}

		err = idx.RemoveID(i.k)
		if err != nil {
			if err == index.ErrNotFound {
				return ErrNotFound
			}
			return err
		}
	}

	d.removed++
	return i.bucket.Delete(i.k)
}

func (d *deleteSink) flush() error {
	if d.removed == 0 {
		return ErrNotFound
	}

	return nil
}

func (d *deleteSink) readOnly() bool {
	return false
}

func newCountSink(node Node, kind interface{}) (*countSink, error) {
	ref := reflect.ValueOf(kind)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return nil, ErrStructPtrNeeded
	}

	return &countSink{
		node: node,
		ref:  ref,
	}, nil
}

type countSink struct {
	node    Node
	ref     reflect.Value
	counter int
}

func (c *countSink) elem() reflect.Value {
	return reflect.New(reflect.Indirect(c.ref).Type())
}

func (c *countSink) bucketName() string {
	return reflect.Indirect(c.ref).Type().Name()
}

func (c *countSink) add(i *item) error {
	c.counter++
	return nil
}

func (c *countSink) flush() error {
	return nil
}

func (c *countSink) readOnly() bool {
	return true
}

func newRawSink() *rawSink {
	return &rawSink{}
}

type rawSink struct {
	results [][]byte
	execFn  func([]byte, []byte) error
}

func (r *rawSink) add(i *item) error {
	if r.execFn != nil {
		err := r.execFn(i.k, i.v)
		if err != nil {
			return err
		}
	} else {
		r.results = append(r.results, i.v)
	}

	return nil
}

func (r *rawSink) bucketName() string {
	return ""
}

func (r *rawSink) flush() error {
	return nil
}

func (r *rawSink) readOnly() bool {
	return true
}

func newEachSink(to interface{}) (*eachSink, error) {
	ref := reflect.ValueOf(to)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Struct {
		return nil, ErrStructPtrNeeded
	}

	return &eachSink{
		ref: ref,
	}, nil
}

type eachSink struct {
	ref    reflect.Value
	execFn func(interface{}) error
}

func (e *eachSink) elem() reflect.Value {
	return reflect.New(reflect.Indirect(e.ref).Type())
}

func (e *eachSink) bucketName() string {
	return reflect.Indirect(e.ref).Type().Name()
}

func (e *eachSink) add(i *item) error {
	return e.execFn(i.value.Interface())
}

func (e *eachSink) flush() error {
	return nil
}

func (e *eachSink) readOnly() bool {
	return true
}
