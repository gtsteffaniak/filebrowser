package fileutils

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/gtsteffaniak/go-logger/logger"
)

// ProgressCallback receives the cumulative number of bytes copied so far.
type ProgressCallback func(bytesCopied int64)

type progressReader struct {
	reader   io.Reader
	ctx      context.Context
	counter  *atomic.Int64
	callback ProgressCallback
	interval int64
	last     int64
}

func newProgressReader(ctx context.Context, r io.Reader, counter *atomic.Int64, interval int64, cb ProgressCallback) *progressReader {
	return &progressReader{
		reader:   r,
		ctx:      ctx,
		counter:  counter,
		callback: cb,
		interval: interval,
		last:     counter.Load(),
	}
}

func (pr *progressReader) Read(p []byte) (int, error) {
	if pr.ctx != nil {
		select {
		case <-pr.ctx.Done():
			return 0, pr.ctx.Err()
		default:
		}
	}
	n, err := pr.reader.Read(p)
	if n > 0 {
		current := pr.counter.Add(int64(n))
		if current-pr.last >= pr.interval {
			pr.callback(current)
			pr.last = current
		}
	}
	return n, err
}

func CopyFileWithProgress(ctx context.Context, source, dest string, cb ProgressCallback) error {
	counter := &atomic.Int64{}
	return copyFileWithCounter(ctx, source, dest, counter, cb)
}

func copyFileWithCounter(ctx context.Context, source, dest string, counter *atomic.Int64, cb ProgressCallback) error {
	srcInfo, err := os.Stat(source)
	if err != nil {
		return err
	}
	sourcePerms := srcInfo.Mode().Perm()

	src, err := os.Open(source)
	if err != nil {
		return err
	}
	defer src.Close()

	err = os.MkdirAll(filepath.Dir(dest), PermDir)
	if err != nil {
		return err
	}

	dst, err := os.OpenFile(dest, os.O_RDWR|os.O_CREATE|os.O_TRUNC, sourcePerms)
	if err != nil {
		return err
	}
	defer dst.Close()

	var reader io.Reader = src
	if cb != nil {
		reader = newProgressReader(ctx, src, counter, 512*1024, cb)
	}

	_, err = io.Copy(dst, reader)
	if err != nil {
		return err
	}

	// Report final bytes for this file so small files aren't missed
	if cb != nil {
		cb(counter.Load())
	}

	err = os.Chmod(dest, sourcePerms)
	if err != nil {
		logger.Debugf("Could not set file permissions for %s: %v", dest, err)
	}

	return nil
}

func CopyDirectoryWithProgress(ctx context.Context, source, dest string, cb ProgressCallback) error {
	counter := &atomic.Int64{}
	return copyDirWithCounter(ctx, source, dest, counter, cb)
}

func copyDirWithCounter(ctx context.Context, source, dest string, counter *atomic.Int64, cb ProgressCallback) error {
	err := os.MkdirAll(dest, PermDir)
	if err != nil {
		return err
	}

	entries, err := os.ReadDir(source)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}
		}

		srcPath := filepath.Join(source, entry.Name())
		destPath := filepath.Join(dest, entry.Name())

		if entry.IsDir() {
			err = copyDirWithCounter(ctx, srcPath, destPath, counter, cb)
		} else {
			err = copyFileWithCounter(ctx, srcPath, destPath, counter, cb)
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func MoveFileWithProgress(ctx context.Context, src, dst string, cb ProgressCallback) error {
	err := os.Rename(src, dst)
	if err == nil {
		// Rename succeeded instantly — no bytes were streamed, but the
		// caller needs to know the full size was "transferred".
		return nil
	}

	// Cross-volume fallback: copy with progress then remove source
	info, statErr := os.Stat(src)
	if statErr != nil {
		return statErr
	}

	if info.IsDir() {
		err = CopyDirectoryWithProgress(ctx, src, dst, cb)
	} else {
		err = CopyFileWithProgress(ctx, src, dst, cb)
	}
	if err != nil {
		return err
	}

	go func() {
		if rmErr := os.RemoveAll(src); rmErr != nil {
			logger.Errorf("os.RemoveAll failed after move: %v", rmErr)
		}
	}()

	return nil
}

func CalculateTotalSize(path string) (int64, error) {
	var total int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total, err
}
