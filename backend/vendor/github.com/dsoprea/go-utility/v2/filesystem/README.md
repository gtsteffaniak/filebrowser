[![GoDoc](https://godoc.org/github.com/dsoprea/go-utility/filesystem?status.svg)](https://godoc.org/github.com/dsoprea/go-utility/filesystem)
[![Build Status](https://travis-ci.org/dsoprea/go-utility.svg?branch=master)](https://travis-ci.org/dsoprea/go-utility)
[![Coverage Status](https://coveralls.io/repos/github/dsoprea/go-utility/badge.svg?branch=master)](https://coveralls.io/github/dsoprea/go-utility?branch=master)
[![Go Report Card](https://goreportcard.com/badge/github.com/dsoprea/go-utility)](https://goreportcard.com/report/github.com/dsoprea/go-utility)

# bounceback

An `io.ReadSeeker` and `io.WriteSeeker` that returns to the right place before
reading or writing. Useful when the same file resource is being reused for reads
or writes throughout that file.

# list_files

A recursive path walker that supports filters.

# seekable_buffer

A memory structure that satisfies `io.ReadWriteSeeker`.

# copy_bytes_between_positions

Given an `io.ReadWriteSeeker`, copy N bytes from one position to an earlier
position.

# read_counter, write_counter

Wrap `io.Reader` and `io.Writer` structs in order to report how many bytes were
transferred.

# readseekwritecloser

Provides the ReadWriteSeekCloser interface that combines a RWS and a Closer.
Also provides a no-op wrapper to augment a plain RWS with a closer.

# boundedreadwriteseek

Wraps a ReadWriteSeeker such that no seeks can be at an offset less than a
specific-offset.

# calculateseek

Provides a reusable function with which to calculate seek offsets.

# progress_wrapper

Provides `io.Reader` and `io.Writer` wrappers that also trigger callbacks after
each call. The reader wrapper also invokes the callback upon EOF.

# does_exist

Check whether a file/directory exists using a file-path.

# graceful_copy

Do a copy but correctly handle short-writes and reads that might return a non-
zero read count *and* EOF.

# readseeker_to_readerat

A wrapper that allows an `io.ReadSeeker` to be used as a `io.ReaderAt`.

# simplefileinfo

An implementation of `os.FileInfo` to support testing.
