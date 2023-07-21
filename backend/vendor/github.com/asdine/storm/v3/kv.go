package storm

import (
	"reflect"

	bolt "go.etcd.io/bbolt"
)

// KeyValueStore can store and fetch values by key
type KeyValueStore interface {
	// Get a value from a bucket
	Get(bucketName string, key interface{}, to interface{}) error
	// Set a key/value pair into a bucket
	Set(bucketName string, key interface{}, value interface{}) error
	// Delete deletes a key from a bucket
	Delete(bucketName string, key interface{}) error
	// GetBytes gets a raw value from a bucket.
	GetBytes(bucketName string, key interface{}) ([]byte, error)
	// SetBytes sets a raw value into a bucket.
	SetBytes(bucketName string, key interface{}, value []byte) error
	// KeyExists reports the presence of a key in a bucket.
	KeyExists(bucketName string, key interface{}) (bool, error)
}

// GetBytes gets a raw value from a bucket.
func (n *node) GetBytes(bucketName string, key interface{}) ([]byte, error) {
	id, err := toBytes(key, n.codec)
	if err != nil {
		return nil, err
	}

	var val []byte
	return val, n.readTx(func(tx *bolt.Tx) error {
		raw, err := n.getBytes(tx, bucketName, id)
		if err != nil {
			return err
		}

		val = make([]byte, len(raw))
		copy(val, raw)
		return nil
	})
}

// GetBytes gets a raw value from a bucket.
func (n *node) getBytes(tx *bolt.Tx, bucketName string, id []byte) ([]byte, error) {
	bucket := n.GetBucket(tx, bucketName)
	if bucket == nil {
		return nil, ErrNotFound
	}

	raw := bucket.Get(id)
	if raw == nil {
		return nil, ErrNotFound
	}

	return raw, nil
}

// SetBytes sets a raw value into a bucket.
func (n *node) SetBytes(bucketName string, key interface{}, value []byte) error {
	if key == nil {
		return ErrNilParam
	}

	id, err := toBytes(key, n.codec)
	if err != nil {
		return err
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.setBytes(tx, bucketName, id, value)
	})
}

func (n *node) setBytes(tx *bolt.Tx, bucketName string, id, data []byte) error {
	bucket, err := n.CreateBucketIfNotExists(tx, bucketName)
	if err != nil {
		return err
	}

	// save node configuration in the bucket
	_, err = newMeta(bucket, n)
	if err != nil {
		return err
	}

	return bucket.Put(id, data)
}

// Get a value from a bucket
func (n *node) Get(bucketName string, key interface{}, to interface{}) error {
	ref := reflect.ValueOf(to)

	if !ref.IsValid() || ref.Kind() != reflect.Ptr {
		return ErrPtrNeeded
	}

	id, err := toBytes(key, n.codec)
	if err != nil {
		return err
	}

	return n.readTx(func(tx *bolt.Tx) error {
		raw, err := n.getBytes(tx, bucketName, id)
		if err != nil {
			return err
		}

		return n.codec.Unmarshal(raw, to)
	})
}

// Set a key/value pair into a bucket
func (n *node) Set(bucketName string, key interface{}, value interface{}) error {
	var data []byte
	var err error
	if value != nil {
		data, err = n.codec.Marshal(value)
		if err != nil {
			return err
		}
	}

	return n.SetBytes(bucketName, key, data)
}

// Delete deletes a key from a bucket
func (n *node) Delete(bucketName string, key interface{}) error {
	id, err := toBytes(key, n.codec)
	if err != nil {
		return err
	}

	return n.readWriteTx(func(tx *bolt.Tx) error {
		return n.delete(tx, bucketName, id)
	})
}

func (n *node) delete(tx *bolt.Tx, bucketName string, id []byte) error {
	bucket := n.GetBucket(tx, bucketName)
	if bucket == nil {
		return ErrNotFound
	}

	return bucket.Delete(id)
}

// KeyExists reports the presence of a key in a bucket.
func (n *node) KeyExists(bucketName string, key interface{}) (bool, error) {
	id, err := toBytes(key, n.codec)
	if err != nil {
		return false, err
	}

	var exists bool
	return exists, n.readTx(func(tx *bolt.Tx) error {
		bucket := n.GetBucket(tx, bucketName)
		if bucket == nil {
			return ErrNotFound
		}

		v := bucket.Get(id)
		if v != nil {
			exists = true
		}

		return nil
	})
}
