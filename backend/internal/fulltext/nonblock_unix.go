//go:build unix

package fulltext

import "syscall"

// oNonBlock avoids blocking when a FIFO replaces a path between stat and open.
const oNonBlock = syscall.O_NONBLOCK
