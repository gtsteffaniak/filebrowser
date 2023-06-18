package storm

import (
	"github.com/asdine/storm/v3/codec"
	bolt "go.etcd.io/bbolt"
)

// A Node in Storm represents the API to a BoltDB bucket.
type Node interface {
	Tx
	TypeStore
	KeyValueStore
	BucketScanner

	// From returns a new Storm node with a new bucket root below the current.
	// All DB operations on the new node will be executed relative to this bucket.
	From(addend ...string) Node

	// Bucket returns the bucket name as a slice from the root.
	// In the normal, simple case this will be empty.
	Bucket() []string

	// GetBucket returns the given bucket below the current node.
	GetBucket(tx *bolt.Tx, children ...string) *bolt.Bucket

	// CreateBucketIfNotExists creates the bucket below the current node if it doesn't
	// already exist.
	CreateBucketIfNotExists(tx *bolt.Tx, bucket string) (*bolt.Bucket, error)

	// WithTransaction returns a New Storm node that will use the given transaction.
	WithTransaction(tx *bolt.Tx) Node

	// Begin starts a new transaction.
	Begin(writable bool) (Node, error)

	// Codec used by this instance of Storm
	Codec() codec.MarshalUnmarshaler

	// WithCodec returns a New Storm Node that will use the given Codec.
	WithCodec(codec codec.MarshalUnmarshaler) Node

	// WithBatch returns a new Storm Node with the batch mode enabled.
	WithBatch(enabled bool) Node
}

// A Node in Storm represents the API to a BoltDB bucket.
type node struct {
	s *DB

	// The root bucket. In the normal, simple case this will be empty.
	rootBucket []string

	// Transaction object. Nil if not in transaction
	tx *bolt.Tx

	// Codec of this node
	codec codec.MarshalUnmarshaler

	// Enable batch mode for read-write transaction, instead of update mode
	batchMode bool
}

// From returns a new Storm Node with a new bucket root below the current.
// All DB operations on the new node will be executed relative to this bucket.
func (n node) From(addend ...string) Node {
	n.rootBucket = append(n.rootBucket, addend...)
	return &n
}

// WithTransaction returns a new Storm Node that will use the given transaction.
func (n node) WithTransaction(tx *bolt.Tx) Node {
	n.tx = tx
	return &n
}

// WithCodec returns a new Storm Node that will use the given Codec.
func (n node) WithCodec(codec codec.MarshalUnmarshaler) Node {
	n.codec = codec
	return &n
}

// WithBatch returns a new Storm Node with the batch mode enabled.
func (n node) WithBatch(enabled bool) Node {
	n.batchMode = enabled
	return &n
}

// Bucket returns the bucket name as a slice from the root.
// In the normal, simple case this will be empty.
func (n *node) Bucket() []string {
	return n.rootBucket
}

// Codec returns the EncodeDecoder used by this instance of Storm
func (n *node) Codec() codec.MarshalUnmarshaler {
	return n.codec
}

// Detects if already in transaction or runs a read write transaction.
// Uses batch mode if enabled.
func (n *node) readWriteTx(fn func(tx *bolt.Tx) error) error {
	if n.tx != nil {
		return fn(n.tx)
	}

	if n.batchMode {
		return n.s.Bolt.Batch(func(tx *bolt.Tx) error {
			return fn(tx)
		})
	}

	return n.s.Bolt.Update(func(tx *bolt.Tx) error {
		return fn(tx)
	})
}

// Detects if already in transaction or runs a read transaction.
func (n *node) readTx(fn func(tx *bolt.Tx) error) error {
	if n.tx != nil {
		return fn(n.tx)
	}

	return n.s.Bolt.View(func(tx *bolt.Tx) error {
		return fn(tx)
	})
}
