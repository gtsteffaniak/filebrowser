package storm

import (
	"reflect"

	bolt "go.etcd.io/bbolt"
)

const (
	metaCodec = "codec"
)

func newMeta(b *bolt.Bucket, n Node) (*meta, error) {
	m := b.Bucket([]byte(metadataBucket))
	if m != nil {
		name := m.Get([]byte(metaCodec))
		if string(name) != n.Codec().Name() {
			return nil, ErrDifferentCodec
		}
		return &meta{
			node:   n,
			bucket: m,
		}, nil
	}

	m, err := b.CreateBucket([]byte(metadataBucket))
	if err != nil {
		return nil, err
	}

	m.Put([]byte(metaCodec), []byte(n.Codec().Name()))
	return &meta{
		node:   n,
		bucket: m,
	}, nil
}

type meta struct {
	node   Node
	bucket *bolt.Bucket
}

func (m *meta) increment(field *fieldConfig) error {
	var err error
	counter := field.IncrementStart

	raw := m.bucket.Get([]byte(field.Name + "counter"))
	if raw != nil {
		counter, err = numberfromb(raw)
		if err != nil {
			return err
		}
		counter++
	}

	raw, err = numbertob(counter)
	if err != nil {
		return err
	}

	err = m.bucket.Put([]byte(field.Name+"counter"), raw)
	if err != nil {
		return err
	}

	field.Value.Set(reflect.ValueOf(counter).Convert(field.Value.Type()))
	field.IsZero = false
	return nil
}
