package storm

import bolt "go.etcd.io/bbolt"

// Tx is a transaction.
type Tx interface {
	// Commit writes all changes to disk.
	Commit() error

	// Rollback closes the transaction and ignores all previous updates.
	Rollback() error
}

// Begin starts a new transaction.
func (n node) Begin(writable bool) (Node, error) {
	var err error

	n.tx, err = n.s.Bolt.Begin(writable)
	if err != nil {
		return nil, err
	}

	return &n, nil
}

// Rollback closes the transaction and ignores all previous updates.
func (n *node) Rollback() error {
	if n.tx == nil {
		return ErrNotInTransaction
	}

	err := n.tx.Rollback()
	if err == bolt.ErrTxClosed {
		return ErrNotInTransaction
	}

	return err
}

// Commit writes all changes to disk.
func (n *node) Commit() error {
	if n.tx == nil {
		return ErrNotInTransaction
	}

	err := n.tx.Commit()
	if err == bolt.ErrTxClosed {
		return ErrNotInTransaction
	}

	return err
}
