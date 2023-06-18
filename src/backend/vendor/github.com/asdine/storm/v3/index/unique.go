package index

import (
	"bytes"

	"github.com/asdine/storm/v3/internal"
	bolt "go.etcd.io/bbolt"
)

// NewUniqueIndex loads a UniqueIndex
func NewUniqueIndex(parent *bolt.Bucket, indexName []byte) (*UniqueIndex, error) {
	var err error
	b := parent.Bucket(indexName)
	if b == nil {
		if !parent.Writable() {
			return nil, ErrNotFound
		}
		b, err = parent.CreateBucket(indexName)
		if err != nil {
			return nil, err
		}
	}

	return &UniqueIndex{
		IndexBucket: b,
		Parent:      parent,
	}, nil
}

// UniqueIndex is an index that references unique values and the corresponding ID.
type UniqueIndex struct {
	Parent      *bolt.Bucket
	IndexBucket *bolt.Bucket
}

// Add a value to the unique index
func (idx *UniqueIndex) Add(value []byte, targetID []byte) error {
	if value == nil || len(value) == 0 {
		return ErrNilParam
	}
	if targetID == nil || len(targetID) == 0 {
		return ErrNilParam
	}

	exists := idx.IndexBucket.Get(value)
	if exists != nil {
		if bytes.Equal(exists, targetID) {
			return nil
		}
		return ErrAlreadyExists
	}

	return idx.IndexBucket.Put(value, targetID)
}

// Remove a value from the unique index
func (idx *UniqueIndex) Remove(value []byte) error {
	return idx.IndexBucket.Delete(value)
}

// RemoveID removes an ID from the unique index
func (idx *UniqueIndex) RemoveID(id []byte) error {
	c := idx.IndexBucket.Cursor()

	for val, ident := c.First(); val != nil; val, ident = c.Next() {
		if bytes.Equal(ident, id) {
			return idx.Remove(val)
		}
	}
	return nil
}

// Get the id corresponding to the given value
func (idx *UniqueIndex) Get(value []byte) []byte {
	return idx.IndexBucket.Get(value)
}

// All returns all the ids corresponding to the given value
func (idx *UniqueIndex) All(value []byte, opts *Options) ([][]byte, error) {
	id := idx.IndexBucket.Get(value)
	if id != nil {
		return [][]byte{id}, nil
	}

	return nil, nil
}

// AllRecords returns all the IDs of this index
func (idx *UniqueIndex) AllRecords(opts *Options) ([][]byte, error) {
	var list [][]byte

	c := internal.Cursor{C: idx.IndexBucket.Cursor(), Reverse: opts != nil && opts.Reverse}

	for val, ident := c.First(); val != nil; val, ident = c.Next() {
		if opts != nil && opts.Skip > 0 {
			opts.Skip--
			continue
		}

		if opts != nil && opts.Limit == 0 {
			break
		}

		if opts != nil && opts.Limit > 0 {
			opts.Limit--
		}

		list = append(list, ident)
	}
	return list, nil
}

// Range returns the ids corresponding to the given range of values
func (idx *UniqueIndex) Range(min []byte, max []byte, opts *Options) ([][]byte, error) {
	var list [][]byte

	c := internal.RangeCursor{
		C:       idx.IndexBucket.Cursor(),
		Reverse: opts != nil && opts.Reverse,
		Min:     min,
		Max:     max,
		CompareFn: func(val, limit []byte) int {
			return bytes.Compare(val, limit)
		},
	}

	for val, ident := c.First(); val != nil && c.Continue(val); val, ident = c.Next() {
		if opts != nil && opts.Skip > 0 {
			opts.Skip--
			continue
		}

		if opts != nil && opts.Limit == 0 {
			break
		}

		if opts != nil && opts.Limit > 0 {
			opts.Limit--
		}

		list = append(list, ident)
	}
	return list, nil
}

// Prefix returns the ids whose values have the given prefix.
func (idx *UniqueIndex) Prefix(prefix []byte, opts *Options) ([][]byte, error) {
	var list [][]byte

	c := internal.PrefixCursor{
		C:       idx.IndexBucket.Cursor(),
		Reverse: opts != nil && opts.Reverse,
		Prefix:  prefix,
	}

	for val, ident := c.First(); val != nil && c.Continue(val); val, ident = c.Next() {
		if opts != nil && opts.Skip > 0 {
			opts.Skip--
			continue
		}

		if opts != nil && opts.Limit == 0 {
			break
		}

		if opts != nil && opts.Limit > 0 {
			opts.Limit--
		}

		list = append(list, ident)
	}
	return list, nil
}

// first returns the first ID of this index
func (idx *UniqueIndex) first() []byte {
	c := idx.IndexBucket.Cursor()

	for val, ident := c.First(); val != nil; val, ident = c.Next() {
		return ident
	}
	return nil
}
