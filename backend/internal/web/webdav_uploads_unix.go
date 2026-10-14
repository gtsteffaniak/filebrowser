//go:build !windows

package web

import "syscall"

// owncloudNoFollowFlag is OR'ed into the open flags for chunk files so a symlink can never
// be followed even if it is swapped in between the Lstat check and the open.
const owncloudNoFollowFlag = syscall.O_NOFOLLOW
