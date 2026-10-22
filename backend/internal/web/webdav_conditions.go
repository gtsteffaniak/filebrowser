package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/webdav"
)

var errDAVPrecondition = errors.New("WebDAV HTTP precondition failed")

// conditionalDAVFS checks HTTP preconditions at the first mutation, after the
// DAV handler has acquired its lock lease and before opening/truncating a file.
// A stat in middleware before ServeHTTP would race with another DAV writer.
type conditionalDAVFS struct {
	webdav.FileSystem
	request *http.Request
	name    string
	once    sync.Once
	err     error
}

func davETag(info os.FileInfo) string {
	return fmt.Sprintf(`"%x%x"`, info.ModTime().UnixNano(), info.Size())
}

func davTagMatches(header, tag string, exists, weak bool) bool {
	if strings.TrimSpace(header) == "*" {
		return exists
	}
	for header != "" {
		header = strings.TrimLeft(header, " \t,")
		prefix := ""
		if strings.HasPrefix(header, "W/") {
			prefix = "W/"
			header = header[2:]
		}
		if !strings.HasPrefix(header, `"`) {
			return false
		}
		end := strings.IndexByte(header[1:], '"')
		if end < 0 {
			return false
		}
		end++
		candidate := header[:end+1]
		if exists && (weak || prefix == "") && candidate == tag {
			return true
		}
		header = header[end+1:]
		if header != "" && !strings.HasPrefix(strings.TrimLeft(header, " \t"), ",") {
			return false
		}
	}
	return false
}

func davCheckConditions(r *http.Request, info os.FileInfo, exists bool) error {
	tag := ""
	if exists {
		tag = davETag(info)
	}
	if match := r.Header.Get("If-Match"); match != "" {
		if !davTagMatches(match, tag, exists, false) {
			return errDAVPrecondition
		}
	} else if value := r.Header.Get("If-Unmodified-Since"); exists && value != "" {
		if stamp, err := http.ParseTime(value); err == nil && info.ModTime().Truncate(time.Second).After(stamp) {
			return errDAVPrecondition
		}
	}
	if match := r.Header.Get("If-None-Match"); match != "" && davTagMatches(match, tag, exists, true) {
		return errDAVPrecondition
	}
	return nil
}

func (f *conditionalDAVFS) check(ctx context.Context) error {
	f.once.Do(func() {
		switch f.request.Method {
		case http.MethodPut, http.MethodDelete, "MOVE", "COPY", "MKCOL", "PROPPATCH":
		default:
			return
		}
		info, err := f.Stat(ctx, f.name)
		if err != nil && !os.IsNotExist(err) {
			f.err = err
			return
		}
		f.err = davCheckConditions(f.request, info, err == nil)
	})
	return f.err
}
func (f *conditionalDAVFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0 {
		if err := f.check(ctx); err != nil {
			return nil, err
		}
	}
	return f.FileSystem.OpenFile(ctx, name, flag, perm)
}
func (f *conditionalDAVFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	if err := f.check(ctx); err != nil {
		return err
	}
	return f.FileSystem.Mkdir(ctx, name, perm)
}
func (f *conditionalDAVFS) RemoveAll(ctx context.Context, name string) error {
	if err := f.check(ctx); err != nil {
		return err
	}
	return f.FileSystem.RemoveAll(ctx, name)
}
func (f *conditionalDAVFS) Rename(ctx context.Context, oldName, newName string) error {
	if err := f.check(ctx); err != nil {
		return err
	}
	return f.FileSystem.Rename(ctx, oldName, newName)
}

// x/net/webdav maps arbitrary filesystem errors to operation-specific statuses.
// Preserve the HTTP precondition status when our filesystem rejected a write.
type conditionalDAVWriter struct {
	http.ResponseWriter
	fs *conditionalDAVFS
}

func (w *conditionalDAVWriter) WriteHeader(status int) {
	if errors.Is(w.fs.err, errDAVPrecondition) {
		status = http.StatusPreconditionFailed
	}
	w.ResponseWriter.WriteHeader(status)
}
