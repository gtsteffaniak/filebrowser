package files

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"golang.org/x/net/webdav"
)

func TestResourceMetadataHooksWithinMutationLease(t *testing.T) {
	for _, operation := range []string{"delete", "move", "copy", "mkdir", "write"} {
		t.Run(operation, func(t *testing.T) {
			idx, root := mutationFixture(t)
			src, dst := filepath.Join(root, "source"), filepath.Join(root, "destination")
			if err := os.WriteFile(src, []byte("original"), 0600); err != nil {
				t.Fatal(err)
			}
			prior := resourceMetadataHooks()
			defer RegisterResourceMetadataHooks(prior)
			before, after := 0, 0
			check := func(key string) error {
				after++
				// Temporary API lease must still exclude a DAV lock from its callback.
				token, err := idx.WebdavLock.Create(time.Now(), webdav.LockDetails{Root: key, Duration: -1})
				if err == nil {
					_ = idx.WebdavLock.Unlock(time.Now(), token)
					t.Fatal("metadata callback escaped mutation lease")
				}
				if !errors.Is(err, webdav.ErrLocked) {
					t.Fatalf("lock: %v", err)
				}
				return nil
			}
			RegisterResourceMetadataHooks(ResourceMetadataHooks{
				Preflight: func() error { before++; return nil },
				Delete:    check,
				Move: func(old, new string) error {
					if old != src || new != dst {
						t.Fatalf("move keys %q %q", old, new)
					}
					return check(new)
				},
				Copy: func(old, new string) error {
					if old != src || new != dst {
						t.Fatalf("copy keys %q %q", old, new)
					}
					return check(new)
				},
			})
			var err error
			switch operation {
			case "delete":
				err = DeleteFiles(idx.Name, "/", "/source", false)
			case "move":
				err = moveResourceImpl(false, idx.Name, idx.Name, src, dst, &Service{})
			case "copy":
				err = CopyResource(false, idx.Name, idx.Name, src, dst)
			case "mkdir":
				err = WriteDirectory("/", utils.FileOptions{Source: idx.Name, Path: "/destination", IsDir: true})
			case "write":
				err = writeFile(idx.Name, "/", "/destination", strings.NewReader("new"))
			}
			if err != nil {
				t.Fatal(err)
			}
			if before != 1 || after != 1 {
				t.Fatalf("callbacks before=%d after=%d", before, after)
			}
		})
	}
}

func TestResourceMetadataPreflightPreventsPhysicalDeletion(t *testing.T) {
	idx, root := mutationFixture(t)
	name := filepath.Join(root, "source")
	if err := os.WriteFile(name, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	prior := resourceMetadataHooks()
	defer RegisterResourceMetadataHooks(prior)
	sentinel := errors.New("metadata unavailable")
	RegisterResourceMetadataHooks(ResourceMetadataHooks{Preflight: func() error { return sentinel }})
	if err := DeleteFiles(idx.Name, "/", "/source", false); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if contents, err := os.ReadFile(name); err != nil || string(contents) != "original" {
		t.Fatalf("physical mutation before preflight: %q %v", contents, err)
	}
}
