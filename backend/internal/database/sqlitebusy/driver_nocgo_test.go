//go:build !cgosql
// +build !cgosql

package sqlitebusy

import (
	"fmt"

	_ "modernc.org/sqlite"
)

const testDriver = "sqlite"

// fakeCodeError mimics the result-code API of modernc's *sqlite.Error (its
// fields are unexported, so it cannot be constructed directly) so that
// extended result codes can be exercised through IsBusyOrLocked and Wrap.
type fakeCodeError int

func (e fakeCodeError) Error() string { return fmt.Sprintf("sqlite error (%d)", int(e)) }
func (e fakeCodeError) Code() int     { return int(e) }

func newCodeError(code int) error { return fakeCodeError(code) }
