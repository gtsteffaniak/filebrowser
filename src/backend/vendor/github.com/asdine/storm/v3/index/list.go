package index

import (
	"bytes"

	"github.com/asdine/storm/v3/internal"
	bolt "go.etcd.io/bbolt"
)

// NewListIndex loads a ListIndex
func NewListIndex(parent *bolt.Bucket, indexName []byte) (*ListIndex, error) {
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

	ids, err := NewUniqueIndex(b, []byte("storm__ids"))
	if err != nil {
		return nil, err
	}

	return &ListIndex{
		IndexBucket: b,
		Parent:      parent,
		IDs:         ids,
	}, nil
}

// ListIndex is an index that references values and the corresponding IDs.
type ListIndex struct {
	Parent      *bolt.Bucket
	IndexBucket *bolt.Bucket
	IDs         *UniqueIndex
}

// Add a value to the list index
func (idx *ListIndex) Add(newValue []byte, targetID []byte) error {
	if newValue == nil || len(newValue) == 0 {
		return ErrNilParam
	}
	if targetID == nil || len(targetID) == 0 {
		return ErrNilParam
	}

	key := idx.IDs.Get(targetID)
	if key != nil {
		err := idx.IndexBucket.Delete(key)
		if err != nil {
			return err
		}

		err = idx.IDs.Remove(targetID)
		if err != nil {
			return err
		}

		key = key[:0]
	}

	key = append(key, newValue...)
	key = append(key, '_')
	key = append(key, '_')
	key = append(key, targetID...)

	err := idx.IDs.Add(targetID, key)
	if err != nil {
		return err
	}

	return idx.IndexBucket.Put(key, targetID)
}

// Remove a value from the unique index
func (idx *ListIndex) Remove(value []byte) error {
	var err error
	var keys [][]byte

	c := idx.IndexBucket.Cursor()
	prefix := generatePrefix(value)

	for k, _ := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, _ = c.Next() {
		keys = append(keys, k)
	}

	for _, k := range keys {
		err = idx.IndexBucket.Delete(k)
		if err != nil {
			return err
		}
	}

	return idx.IDs.RemoveID(value)
}

// RemoveID removes an ID from the list index
func (idx *ListIndex) RemoveID(targetID []byte) error {
	value := idx.IDs.Get(targetID)
	if value == nil {
		return nil
	}

	err := idx.IndexBucket.Delete(value)
	if err != nil {
		return err
	}

	return idx.IDs.Remove(targetID)
}

// Get the first ID corresponding to the given value
func (idx *ListIndex) Get(value []byte) []byte {
	c := idx.IndexBucket.Cursor()
	prefix := generatePrefix(value)

	for k, id := c.Seek(prefix); bytes.HasPrefix(k, prefix); k, id = c.Next() {
		return id
	}

	return nil
}

// All the IDs corresponding to the given value
func (idx *ListIndex) All(value []byte, opts *Options) ([][]byte, error) {
	var list [][]byte
	c := idx.IndexBucket.Cursor()
	cur := internal.Cursor{C: c, Reverse: opts != nil && opts.Reverse}

	prefix := generatePrefix(value)

	k, id := c.Seek(prefix)
	if cur.Reverse {
		var count int
		kc := k
		idc := id
		for ; kc != nil && bytes.HasPrefix(kc, prefix); kc, idc = c.Next() {
			count++
			k, id = kc, idc
		}
		if kc != nil {
			k, id = c.Prev()
		}
		list = make([][]byte, 0, count)
	}

	for ; bytes.HasPrefix(k, prefix); k, id = cur.Next() {
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

		list = append(list, id)
	}

	return list, nil
}

// AllRecords returns all the IDs of this index
func (idx *ListIndex) AllRecords(opts *Options) ([][]byte, error) {
	var list [][]byte

	c := internal.Cursor{C: idx.IndexBucket.Cursor(), Reverse: opts != nil && opts.Reverse}

	for k, id := c.First(); k != nil; k, id = c.Next() {
		if id == nil || bytes.Equal(k, []byte("storm__ids")) {
			continue
		}

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

		list = append(list, id)
	}

	return list, nil
}

// Range returns the ids corresponding to the given range of values
func (idx *ListIndex) Range(min []byte, max []byte, opts *Options) ([][]byte, error) {
	var list [][]byte

	c := internal.RangeCursor{
		C:       idx.IndexBucket.Cursor(),
		Reverse: opts != nil && opts.Reverse,
		Min:     min,
		Max:     max,
		CompareFn: func(val, limit []byte) int {
			pos := bytes.LastIndex(val, []byte("__"))
			return bytes.Compare(val[:pos], limit)
		},
	}

	for k, id := c.First(); c.Continue(k); k, id = c.Next() {
		if id == nil || bytes.Equal(k, []byte("storm__ids")) {
			continue
		}

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

		list = append(list, id)
	}

	return list, nil
}

// Prefix returns the ids whose values have the given prefix.
func (idx *ListIndex) Prefix(prefix []byte, opts *Options) ([][]byte, error) {
	var list [][]byte

	c := internal.PrefixCursor{
		C:       idx.IndexBucket.Cursor(),
		Reverse: opts != nil && opts.Reverse,
		Prefix:  prefix,
	}

	for k, id := c.First(); k != nil && c.Continue(k); k, id = c.Next() {
		if id == nil || bytes.Equal(k, []byte("storm__ids")) {
			continue
		}

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

		list = append(list, id)
	}
	return list, nil
}

func generatePrefix(value []byte) []byte {
	prefix := make([]byte, len(value)+2)
	var i int
	for i = range value {
		prefix[i] = value[i]
	}
	prefix[i+1] = '_'
	prefix[i+2] = '_'
	return prefix
}
