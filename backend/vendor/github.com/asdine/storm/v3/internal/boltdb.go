package internal

import (
	"bytes"

	bolt "go.etcd.io/bbolt"
)

// Cursor that can be reversed
type Cursor struct {
	C       *bolt.Cursor
	Reverse bool
}

// First element
func (c *Cursor) First() ([]byte, []byte) {
	if c.Reverse {
		return c.C.Last()
	}

	return c.C.First()
}

// Next element
func (c *Cursor) Next() ([]byte, []byte) {
	if c.Reverse {
		return c.C.Prev()
	}

	return c.C.Next()
}

// RangeCursor that can be reversed
type RangeCursor struct {
	C         *bolt.Cursor
	Reverse   bool
	Min       []byte
	Max       []byte
	CompareFn func([]byte, []byte) int
}

// First element
func (c *RangeCursor) First() ([]byte, []byte) {
	if c.Reverse {
		k, v := c.C.Seek(c.Max)

		// If Seek doesn't find a key it goes to the next.
		// If so, we need to get the previous one to avoid
		// including bigger values. #218
		if !bytes.HasPrefix(k, c.Max) && k != nil {
			k, v = c.C.Prev()
		}

		return k, v
	}

	return c.C.Seek(c.Min)
}

// Next element
func (c *RangeCursor) Next() ([]byte, []byte) {
	if c.Reverse {
		return c.C.Prev()
	}

	return c.C.Next()
}

// Continue tells if the loop needs to continue
func (c *RangeCursor) Continue(val []byte) bool {
	if c.Reverse {
		return val != nil && c.CompareFn(val, c.Min) >= 0
	}

	return val != nil && c.CompareFn(val, c.Max) <= 0
}

// PrefixCursor that can be reversed
type PrefixCursor struct {
	C       *bolt.Cursor
	Reverse bool
	Prefix  []byte
}

// First element
func (c *PrefixCursor) First() ([]byte, []byte) {
	var k, v []byte

	for k, v = c.C.First(); k != nil && !bytes.HasPrefix(k, c.Prefix); k, v = c.C.Next() {
	}

	if k == nil {
		return nil, nil
	}

	if c.Reverse {
		kc, vc := k, v
		for ; kc != nil && bytes.HasPrefix(kc, c.Prefix); kc, vc = c.C.Next() {
			k, v = kc, vc
		}
		if kc != nil {
			k, v = c.C.Prev()
		}
	}

	return k, v
}

// Next element
func (c *PrefixCursor) Next() ([]byte, []byte) {
	if c.Reverse {
		return c.C.Prev()
	}

	return c.C.Next()
}

// Continue tells if the loop needs to continue
func (c *PrefixCursor) Continue(val []byte) bool {
	return val != nil && bytes.HasPrefix(val, c.Prefix)
}
