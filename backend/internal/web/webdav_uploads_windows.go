//go:build windows

package web

// Windows has no O_NOFOLLOW; openOwncloudChunk's Lstat + SameFile check still prevents a
// swapped-in symlink from being read as a chunk.
const owncloudNoFollowFlag = 0
