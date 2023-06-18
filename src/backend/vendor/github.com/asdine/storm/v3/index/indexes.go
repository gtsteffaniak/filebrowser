// Package index contains Index engines used to store values and their corresponding IDs
package index

// Index interface
type Index interface {
	Add(value []byte, targetID []byte) error
	Remove(value []byte) error
	RemoveID(id []byte) error
	Get(value []byte) []byte
	All(value []byte, opts *Options) ([][]byte, error)
	AllRecords(opts *Options) ([][]byte, error)
	Range(min []byte, max []byte, opts *Options) ([][]byte, error)
	Prefix(prefix []byte, opts *Options) ([][]byte, error)
}
