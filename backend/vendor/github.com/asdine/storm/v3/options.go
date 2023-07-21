package storm

import (
	"os"

	"github.com/asdine/storm/v3/codec"
	"github.com/asdine/storm/v3/index"
	bolt "go.etcd.io/bbolt"
)

// BoltOptions used to pass options to BoltDB.
func BoltOptions(mode os.FileMode, options *bolt.Options) func(*Options) error {
	return func(opts *Options) error {
		opts.boltMode = mode
		opts.boltOptions = options
		return nil
	}
}

// Codec used to set a custom encoder and decoder. The default is JSON.
func Codec(c codec.MarshalUnmarshaler) func(*Options) error {
	return func(opts *Options) error {
		opts.codec = c
		return nil
	}
}

// Batch enables the use of batch instead of update for read-write transactions.
func Batch() func(*Options) error {
	return func(opts *Options) error {
		opts.batchMode = true
		return nil
	}
}

// Root used to set the root bucket. See also the From method.
func Root(root ...string) func(*Options) error {
	return func(opts *Options) error {
		opts.rootBucket = root
		return nil
	}
}

// UseDB allows Storm to use an existing open Bolt.DB.
// Warning: storm.DB.Close() will close the bolt.DB instance.
func UseDB(b *bolt.DB) func(*Options) error {
	return func(opts *Options) error {
		opts.path = b.Path()
		opts.bolt = b
		return nil
	}
}

// Limit sets the maximum number of records to return
func Limit(limit int) func(*index.Options) {
	return func(opts *index.Options) {
		opts.Limit = limit
	}
}

// Skip sets the number of records to skip
func Skip(offset int) func(*index.Options) {
	return func(opts *index.Options) {
		opts.Skip = offset
	}
}

// Reverse will return the results in descending order
func Reverse() func(*index.Options) {
	return func(opts *index.Options) {
		opts.Reverse = true
	}
}

// Options are used to customize the way Storm opens a database.
type Options struct {
	// Handles encoding and decoding of objects
	codec codec.MarshalUnmarshaler

	// Bolt file mode
	boltMode os.FileMode

	// Bolt options
	boltOptions *bolt.Options

	// Enable batch mode for read-write transaction, instead of update mode
	batchMode bool

	// The root bucket name
	rootBucket []string

	// Path of the database file
	path string

	// Bolt is still easily accessible
	bolt *bolt.DB
}
