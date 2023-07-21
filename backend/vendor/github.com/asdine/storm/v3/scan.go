package storm

import (
	"bytes"

	bolt "go.etcd.io/bbolt"
)

// A BucketScanner scans a Node for a list of buckets
type BucketScanner interface {
	// PrefixScan scans the root buckets for keys matching the given prefix.
	PrefixScan(prefix string) []Node
	// PrefixScan scans the buckets in this node for keys matching the given prefix.
	RangeScan(min, max string) []Node
}

// PrefixScan scans the buckets in this node for keys matching the given prefix.
func (n *node) PrefixScan(prefix string) []Node {
	if n.tx != nil {
		return n.prefixScan(n.tx, prefix)
	}

	var nodes []Node

	n.readTx(func(tx *bolt.Tx) error {
		nodes = n.prefixScan(tx, prefix)
		return nil
	})

	return nodes
}

func (n *node) prefixScan(tx *bolt.Tx, prefix string) []Node {
	var (
		prefixBytes = []byte(prefix)
		nodes       []Node
		c           = n.cursor(tx)
	)

	if c == nil {
		return nil
	}

	for k, v := c.Seek(prefixBytes); k != nil && bytes.HasPrefix(k, prefixBytes); k, v = c.Next() {
		if v != nil {
			continue
		}

		nodes = append(nodes, n.From(string(k)))
	}

	return nodes
}

// RangeScan scans the buckets in this node  over a range such as a sortable time range.
func (n *node) RangeScan(min, max string) []Node {
	if n.tx != nil {
		return n.rangeScan(n.tx, min, max)
	}

	var nodes []Node

	n.readTx(func(tx *bolt.Tx) error {
		nodes = n.rangeScan(tx, min, max)
		return nil
	})

	return nodes
}

func (n *node) rangeScan(tx *bolt.Tx, min, max string) []Node {
	var (
		minBytes = []byte(min)
		maxBytes = []byte(max)
		nodes    []Node
		c        = n.cursor(tx)
	)

	for k, v := c.Seek(minBytes); k != nil && bytes.Compare(k, maxBytes) <= 0; k, v = c.Next() {
		if v != nil {
			continue
		}

		nodes = append(nodes, n.From(string(k)))
	}

	return nodes

}

func (n *node) cursor(tx *bolt.Tx) *bolt.Cursor {
	var c *bolt.Cursor

	if len(n.rootBucket) > 0 {
		b := n.GetBucket(tx)
		if b == nil {
			return nil
		}
		c = b.Cursor()
	} else {
		c = tx.Cursor()
	}

	return c
}
