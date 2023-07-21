package storm

import (
	"bytes"
	"encoding/binary"
	"time"

	"github.com/asdine/storm/v3/codec"
	"github.com/asdine/storm/v3/codec/json"
	bolt "go.etcd.io/bbolt"
)

const (
	dbinfo         = "__storm_db"
	metadataBucket = "__storm_metadata"
)

// Defaults to json
var defaultCodec = json.Codec

// Open opens a database at the given path with optional Storm options.
func Open(path string, stormOptions ...func(*Options) error) (*DB, error) {
	var err error

	var opts Options
	for _, option := range stormOptions {
		if err = option(&opts); err != nil {
			return nil, err
		}
	}

	s := DB{
		Bolt: opts.bolt,
	}

	n := node{
		s:          &s,
		codec:      opts.codec,
		batchMode:  opts.batchMode,
		rootBucket: opts.rootBucket,
	}

	if n.codec == nil {
		n.codec = defaultCodec
	}

	if opts.boltMode == 0 {
		opts.boltMode = 0600
	}

	if opts.boltOptions == nil {
		opts.boltOptions = &bolt.Options{Timeout: 1 * time.Second}
	}

	s.Node = &n

	// skip if UseDB option is used
	if s.Bolt == nil {
		s.Bolt, err = bolt.Open(path, opts.boltMode, opts.boltOptions)
		if err != nil {
			return nil, err
		}
	}

	err = s.checkVersion()
	if err != nil {
		return nil, err
	}

	return &s, nil
}

// DB is the wrapper around BoltDB. It contains an instance of BoltDB and uses it to perform all the
// needed operations
type DB struct {
	// The root node that points to the root bucket.
	Node

	// Bolt is still easily accessible
	Bolt *bolt.DB
}

// Close the database
func (s *DB) Close() error {
	return s.Bolt.Close()
}

func (s *DB) checkVersion() error {
	var v string
	err := s.Get(dbinfo, "version", &v)
	if err != nil && err != ErrNotFound {
		return err
	}

	// for now, we only set the current version if it doesn't exist.
	// v1 and v2 database files are compatible.
	if v == "" {
		return s.Set(dbinfo, "version", Version)
	}

	return nil
}

// toBytes turns an interface into a slice of bytes
func toBytes(key interface{}, codec codec.MarshalUnmarshaler) ([]byte, error) {
	if key == nil {
		return nil, nil
	}
	switch t := key.(type) {
	case []byte:
		return t, nil
	case string:
		return []byte(t), nil
	case int:
		return numbertob(int64(t))
	case uint:
		return numbertob(uint64(t))
	case int8, int16, int32, int64, uint8, uint16, uint32, uint64:
		return numbertob(t)
	default:
		return codec.Marshal(key)
	}
}

func numbertob(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	err := binary.Write(&buf, binary.BigEndian, v)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func numberfromb(raw []byte) (int64, error) {
	r := bytes.NewReader(raw)
	var to int64
	err := binary.Read(r, binary.BigEndian, &to)
	if err != nil {
		return 0, err
	}
	return to, nil
}
