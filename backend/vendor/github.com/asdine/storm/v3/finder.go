package storm

import (
	"fmt"
	"reflect"

	"github.com/asdine/storm/v3/index"
	"github.com/asdine/storm/v3/q"
	bolt "go.etcd.io/bbolt"
)

// A Finder can fetch types from BoltDB.
type Finder interface {
	// One returns one record by the specified index
	One(fieldName string, value interface{}, to interface{}) error

	// Find returns one or more records by the specified index
	Find(fieldName string, value interface{}, to interface{}, options ...func(q *index.Options)) error

	// AllByIndex gets all the records of a bucket that are indexed in the specified index
	AllByIndex(fieldName string, to interface{}, options ...func(*index.Options)) error

	// All gets all the records of a bucket.
	// If there are no records it returns no error and the 'to' parameter is set to an empty slice.
	All(to interface{}, options ...func(*index.Options)) error

	// Select a list of records that match a list of matchers. Doesn't use indexes.
	Select(matchers ...q.Matcher) Query

	// Range returns one or more records by the specified index within the specified range
	Range(fieldName string, min, max, to interface{}, options ...func(*index.Options)) error

	// Prefix returns one or more records whose given field starts with the specified prefix.
	Prefix(fieldName string, prefix string, to interface{}, options ...func(*index.Options)) error

	// Count counts all the records of a bucket
	Count(data interface{}) (int, error)
}

// One returns one record by the specified index
func (n *node) One(fieldName string, value interface{}, to interface{}) error {
	sink, err := newFirstSink(n, to)
	if err != nil {
		return err
	}

	bucketName := sink.bucketName()
	if bucketName == "" {
		return ErrNoName
	}

	if fieldName == "" {
		return ErrNotFound
	}

	ref := reflect.Indirect(sink.ref)
	cfg, err := extractSingleField(&ref, fieldName)
	if err != nil {
		return err
	}

	field, ok := cfg.Fields[fieldName]
	if !ok || (!field.IsID && field.Index == "") {
		query := newQuery(n, q.StrictEq(fieldName, value))
		query.Limit(1)

		if n.tx != nil {
			err = query.query(n.tx, sink)
		} else {
			err = n.s.Bolt.View(func(tx *bolt.Tx) error {
				return query.query(tx, sink)
			})
		}

		if err != nil {
			return err
		}

		return sink.flush()
	}

	val, err := toBytes(value, n.codec)
	if err != nil {
		return err
	}

	return n.readTx(func(tx *bolt.Tx) error {
		return n.one(tx, bucketName, fieldName, cfg, to, val, field.IsID)
	})
}

func (n *node) one(tx *bolt.Tx, bucketName, fieldName string, cfg *structConfig, to interface{}, val []byte, skipIndex bool) error {
	bucket := n.GetBucket(tx, bucketName)
	if bucket == nil {
		return ErrNotFound
	}

	var id []byte
	if !skipIndex {
		idx, err := getIndex(bucket, cfg.Fields[fieldName].Index, fieldName)
		if err != nil {
			if err == index.ErrNotFound {
				return ErrNotFound
			}
			return err
		}

		id = idx.Get(val)
	} else {
		id = val
	}

	if id == nil {
		return ErrNotFound
	}

	raw := bucket.Get(id)
	if raw == nil {
		return ErrNotFound
	}

	return n.codec.Unmarshal(raw, to)
}

// Find returns one or more records by the specified index
func (n *node) Find(fieldName string, value interface{}, to interface{}, options ...func(q *index.Options)) error {
	sink, err := newListSink(n, to)
	if err != nil {
		return err
	}
	bucketName := sink.bucketName()
	if bucketName == "" {
		return ErrNoName
	}

	ref := reflect.Indirect(reflect.New(sink.elemType))
	cfg, err := extractSingleField(&ref, fieldName)
	if err != nil {
		return err
	}

	opts := index.NewOptions()
	for _, fn := range options {
		fn(opts)
	}

	field, ok := cfg.Fields[fieldName]
	if !ok || (!field.IsID && (field.Index == "" || value == nil)) {
		query := newQuery(n, q.Eq(fieldName, value))
		query.Skip(opts.Skip).Limit(opts.Limit)

		if opts.Reverse {
			query.Reverse()
		}

		err = n.readTx(func(tx *bolt.Tx) error {
			return query.query(tx, sink)
		})

		if err != nil {
			return err
		}

		return sink.flush()
	}

	val, err := toBytes(value, n.codec)
	if err != nil {
		return err
	}

	return n.readTx(func(tx *bolt.Tx) error {
		return n.find(tx, bucketName, fieldName, cfg, sink, val, opts)
	})
}

func (n *node) find(tx *bolt.Tx, bucketName, fieldName string, cfg *structConfig, sink *listSink, val []byte, opts *index.Options) error {
	bucket := n.GetBucket(tx, bucketName)
	if bucket == nil {
		return ErrNotFound
	}
	idx, err := getIndex(bucket, cfg.Fields[fieldName].Index, fieldName)
	if err != nil {
		return err
	}

	list, err := idx.All(val, opts)
	if err != nil {
		if err == index.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	sink.results = reflect.MakeSlice(reflect.Indirect(sink.ref).Type(), len(list), len(list))

	sorter := newSorter(n, sink)
	for i := range list {
		raw := bucket.Get(list[i])
		if raw == nil {
			return ErrNotFound
		}

		if _, err := sorter.filter(nil, bucket, list[i], raw); err != nil {
			return err
		}
	}

	return sorter.flush()
}

// AllByIndex gets all the records of a bucket that are indexed in the specified index
func (n *node) AllByIndex(fieldName string, to interface{}, options ...func(*index.Options)) error {
	if fieldName == "" {
		return n.All(to, options...)
	}

	ref := reflect.ValueOf(to)

	if ref.Kind() != reflect.Ptr || ref.Elem().Kind() != reflect.Slice {
		return ErrSlicePtrNeeded
	}

	typ := reflect.Indirect(ref).Type().Elem()

	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	newElem := reflect.New(typ)

	cfg, err := extract(&newElem)
	if err != nil {
		return err
	}

	if cfg.ID.Name == fieldName {
		return n.All(to, options...)
	}

	opts := index.NewOptions()
	for _, fn := range options {
		fn(opts)
	}

	return n.readTx(func(tx *bolt.Tx) error {
		return n.allByIndex(tx, fieldName, cfg, &ref, opts)
	})
}

func (n *node) allByIndex(tx *bolt.Tx, fieldName string, cfg *structConfig, ref *reflect.Value, opts *index.Options) error {
	bucket := n.GetBucket(tx, cfg.Name)
	if bucket == nil {
		return ErrNotFound
	}

	fieldCfg, ok := cfg.Fields[fieldName]
	if !ok {
		return ErrNotFound
	}

	idx, err := getIndex(bucket, fieldCfg.Index, fieldName)
	if err != nil {
		return err
	}

	list, err := idx.AllRecords(opts)
	if err != nil {
		if err == index.ErrNotFound {
			return ErrNotFound
		}
		return err
	}

	results := reflect.MakeSlice(reflect.Indirect(*ref).Type(), len(list), len(list))

	for i := range list {
		raw := bucket.Get(list[i])
		if raw == nil {
			return ErrNotFound
		}

		err = n.codec.Unmarshal(raw, results.Index(i).Addr().Interface())
		if err != nil {
			return err
		}
	}

	reflect.Indirect(*ref).Set(results)
	return nil
}

// All gets all the records of a bucket.
// If there are no records it returns no error and the 'to' parameter is set to an empty slice.
func (n *node) All(to interface{}, options ...func(*index.Options)) error {
	opts := index.NewOptions()
	for _, fn := range options {
		fn(opts)
	}

	query := newQuery(n, nil).Limit(opts.Limit).Skip(opts.Skip)
	if opts.Reverse {
		query.Reverse()
	}

	err := query.Find(to)
	if err != nil && err != ErrNotFound {
		return err
	}

	if err == ErrNotFound {
		ref := reflect.ValueOf(to)
		results := reflect.MakeSlice(reflect.Indirect(ref).Type(), 0, 0)
		reflect.Indirect(ref).Set(results)
	}
	return nil
}

// Range returns one or more records by the specified index within the specified range
func (n *node) Range(fieldName string, min, max, to interface{}, options ...func(*index.Options)) error {
	sink, err := newListSink(n, to)
	if err != nil {
		return err
	}

	bucketName := sink.bucketName()
	if bucketName == "" {
		return ErrNoName
	}

	ref := reflect.Indirect(reflect.New(sink.elemType))
	cfg, err := extractSingleField(&ref, fieldName)
	if err != nil {
		return err
	}

	opts := index.NewOptions()
	for _, fn := range options {
		fn(opts)
	}

	field, ok := cfg.Fields[fieldName]
	if !ok || (!field.IsID && field.Index == "") {
		query := newQuery(n, q.And(q.Gte(fieldName, min), q.Lte(fieldName, max)))
		query.Skip(opts.Skip).Limit(opts.Limit)

		if opts.Reverse {
			query.Reverse()
		}

		err = n.readTx(func(tx *bolt.Tx) error {
			return query.query(tx, sink)
		})

		if err != nil {
			return err
		}

		return sink.flush()
	}

	mn, err := toBytes(min, n.codec)
	if err != nil {
		return err
	}

	mx, err := toBytes(max, n.codec)
	if err != nil {
		return err
	}

	return n.readTx(func(tx *bolt.Tx) error {
		return n.rnge(tx, bucketName, fieldName, cfg, sink, mn, mx, opts)
	})
}

func (n *node) rnge(tx *bolt.Tx, bucketName, fieldName string, cfg *structConfig, sink *listSink, min, max []byte, opts *index.Options) error {
	bucket := n.GetBucket(tx, bucketName)
	if bucket == nil {
		reflect.Indirect(sink.ref).SetLen(0)
		return nil
	}

	idx, err := getIndex(bucket, cfg.Fields[fieldName].Index, fieldName)
	if err != nil {
		return err
	}

	list, err := idx.Range(min, max, opts)
	if err != nil {
		return err
	}

	sink.results = reflect.MakeSlice(reflect.Indirect(sink.ref).Type(), len(list), len(list))
	sorter := newSorter(n, sink)
	for i := range list {
		raw := bucket.Get(list[i])
		if raw == nil {
			return ErrNotFound
		}

		if _, err := sorter.filter(nil, bucket, list[i], raw); err != nil {
			return err
		}
	}

	return sorter.flush()
}

// Prefix returns one or more records whose given field starts with the specified prefix.
func (n *node) Prefix(fieldName string, prefix string, to interface{}, options ...func(*index.Options)) error {
	sink, err := newListSink(n, to)
	if err != nil {
		return err
	}

	bucketName := sink.bucketName()
	if bucketName == "" {
		return ErrNoName
	}

	ref := reflect.Indirect(reflect.New(sink.elemType))
	cfg, err := extractSingleField(&ref, fieldName)
	if err != nil {
		return err
	}

	opts := index.NewOptions()
	for _, fn := range options {
		fn(opts)
	}

	field, ok := cfg.Fields[fieldName]
	if !ok || (!field.IsID && field.Index == "") {
		query := newQuery(n, q.Re(fieldName, fmt.Sprintf("^%s", prefix)))
		query.Skip(opts.Skip).Limit(opts.Limit)

		if opts.Reverse {
			query.Reverse()
		}

		err = n.readTx(func(tx *bolt.Tx) error {
			return query.query(tx, sink)
		})

		if err != nil {
			return err
		}

		return sink.flush()
	}

	prfx, err := toBytes(prefix, n.codec)
	if err != nil {
		return err
	}

	return n.readTx(func(tx *bolt.Tx) error {
		return n.prefix(tx, bucketName, fieldName, cfg, sink, prfx, opts)
	})
}

func (n *node) prefix(tx *bolt.Tx, bucketName, fieldName string, cfg *structConfig, sink *listSink, prefix []byte, opts *index.Options) error {
	bucket := n.GetBucket(tx, bucketName)
	if bucket == nil {
		reflect.Indirect(sink.ref).SetLen(0)
		return nil
	}

	idx, err := getIndex(bucket, cfg.Fields[fieldName].Index, fieldName)
	if err != nil {
		return err
	}

	list, err := idx.Prefix(prefix, opts)
	if err != nil {
		return err
	}

	sink.results = reflect.MakeSlice(reflect.Indirect(sink.ref).Type(), len(list), len(list))
	sorter := newSorter(n, sink)
	for i := range list {
		raw := bucket.Get(list[i])
		if raw == nil {
			return ErrNotFound
		}

		if _, err := sorter.filter(nil, bucket, list[i], raw); err != nil {
			return err
		}
	}

	return sorter.flush()
}

// Count counts all the records of a bucket
func (n *node) Count(data interface{}) (int, error) {
	return n.Select().Count(data)
}
