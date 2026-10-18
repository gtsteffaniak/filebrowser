package web

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/app"
	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func setupExtractAccessPolicyTest(t *testing.T) (destDir, sourceRoot, sourceName, username string) {
	t.Helper()
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "access.sqlite")
	if _, err := state.Initialize(dbPath); err != nil {
		t.Fatal(err)
	}
	app.MustWireServices(state.Default())
	t.Cleanup(func() { state.Close() })

	sourceRoot = filepath.Join(tempDir, "data")
	destDir = filepath.Join(sourceRoot, "dest")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatal(err)
	}

	sourceName = "data"
	settings.Config.Server.SourceMap = map[string]*settings.Source{
		sourceRoot: {Path: sourceRoot, Name: sourceName},
	}
	settings.Config.Server.NameToSource = map[string]*settings.Source{
		sourceName: settings.Config.Server.SourceMap[sourceRoot],
	}
	settings.InitializeUserResolvers()

	username = "extract-user"
	user := &users.User{
		ID: 1,
		FrontendUser: users.FrontendUser{Username: username},
	}
	if err := state.CreateUser(user, ""); err != nil {
		t.Fatal(err)
	}
	if err := state.AllowUser(sourceRoot, utils.IndexPathFromNormalized("/dest", true), username); err != nil {
		t.Fatal(err)
	}

	indexing.SetTestIndex(sourceName, sourceRoot)
	t.Cleanup(func() { indexing.ClearTestIndices() })

	return destDir, sourceRoot, sourceName, username
}

func testExtractPolicy(t *testing.T, destDir, sourceName, username string, perms users.SourceFilePermissions) *extractAccessPolicy {
	idx := indexing.GetIndex(sourceName)
	if idx == nil {
		t.Fatal("missing index")
	}
	return &extractAccessPolicy{
		idx:             idx,
		destRealRoot:    destDir,
		destIndexPrefix: "/dest",
		username:        username,
		perms:           perms,
	}
}

func writeTestZip(t *testing.T, path string, write func(zw *zip.Writer) error) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if err := write(zw); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeTestTarGz(t *testing.T, path string, write func(tw *tar.Writer) error) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	if err := write(tw); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func fullPerms() users.SourceFilePermissions {
	return users.SourceFilePermissions{
		View: true, Download: true, Modify: true, Create: true, Delete: true,
	}
}

func TestExtractZipWithEncoding_accessPolicy(t *testing.T) {
	t.Run("allowed creation", func(t *testing.T) {
		destDir, _, sourceName, username := setupExtractAccessPolicyTest(t)
		zipPath := filepath.Join(t.TempDir(), "ok.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			w, err := zw.Create("new.txt")
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("ok"))
			return err
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		if err := extractZipWithEncoding(zipPath, destDir, "", policy); err != nil {
			t.Fatalf("extract: %v", err)
		}
	})

	t.Run("denied implicit parent directory", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/blocked", true), username); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "parent.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			w, err := zw.Create("blocked/child.txt")
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("nope"))
			return err
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		if err := extractZipWithEncoding(zipPath, destDir, "", policy); err == nil {
			t.Fatal("expected access error for implicit parent")
		}
	})

	t.Run("denied modification", func(t *testing.T) {
		destDir, _, sourceName, username := setupExtractAccessPolicyTest(t)
		existing := filepath.Join(destDir, "existing.txt")
		if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "mod.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			w, err := zw.Create("existing.txt")
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("new"))
			return err
		})
		noModify := fullPerms()
		noModify.Modify = false
		policy := testExtractPolicy(t, destDir, sourceName, username, noModify)
		if err := extractZipWithEncoding(zipPath, destDir, "", policy); err == nil {
			t.Fatal("expected modify permission error")
		}
	})

	t.Run("denied write through symlink path component", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		blocked := filepath.Join(destDir, "blocked")
		if err := os.MkdirAll(blocked, 0o755); err != nil {
			t.Fatal(err)
		}
		alias := filepath.Join(destDir, "alias")
		if err := os.Symlink("blocked", alias); err != nil {
			t.Fatal(err)
		}
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/blocked", true), username); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "symlink.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			w, err := zw.Create("alias/secret.txt")
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("nope"))
			return err
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		if err := extractZipWithEncoding(zipPath, destDir, "", policy); err == nil {
			t.Fatal("expected access denied for resolved symlink target")
		}
	})

	t.Run("symlink entry requires policy", func(t *testing.T) {
		destDir, _, sourceName, username := setupExtractAccessPolicyTest(t)
		zipPath := filepath.Join(t.TempDir(), "link.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			hdr := &zip.FileHeader{Name: "thelink", Method: zip.Store}
			hdr.SetMode(0o777 | os.ModeSymlink)
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("target.txt"))
			return err
		})
		if err := os.WriteFile(filepath.Join(destDir, "target.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		noCreate := fullPerms()
		noCreate.Create = false
		policy := testExtractPolicy(t, destDir, sourceName, username, noCreate)
		if err := extractZipWithEncoding(zipPath, destDir, "", policy); err == nil {
			t.Fatal("expected create permission error for symlink entry")
		}
	})
}

func TestExtractTarGz_accessPolicy(t *testing.T) {
	t.Run("allowed creation", func(t *testing.T) {
		destDir, _, sourceName, username := setupExtractAccessPolicyTest(t)
		archivePath := filepath.Join(t.TempDir(), "ok.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			if err := tw.WriteHeader(&tar.Header{Name: "new.txt", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg}); err != nil {
				return err
			}
			_, err := io.WriteString(tw, "ok")
			return err
		})
		if err := extractTarGz(archivePath, destDir, testExtractPolicy(t, destDir, sourceName, username, fullPerms())); err != nil {
			t.Fatalf("extract: %v", err)
		}
	})

	t.Run("denied implicit parent directory", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/blocked", true), username); err != nil {
			t.Fatal(err)
		}
		archivePath := filepath.Join(t.TempDir(), "parent.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			if err := tw.WriteHeader(&tar.Header{Name: "blocked/child.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg}); err != nil {
				return err
			}
			_, err := io.WriteString(tw, "nope")
			return err
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		if err := extractTarGz(archivePath, destDir, policy); err == nil {
			t.Fatal("expected access error for implicit parent")
		}
	})

	t.Run("symlink entry requires policy", func(t *testing.T) {
		destDir, _, sourceName, username := setupExtractAccessPolicyTest(t)
		archivePath := filepath.Join(t.TempDir(), "link.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			return tw.WriteHeader(&tar.Header{
				Name:     "thelink",
				Mode:     0o777,
				Typeflag: tar.TypeSymlink,
				Linkname: "target.txt",
			})
		})
		if err := os.WriteFile(filepath.Join(destDir, "target.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		noCreate := fullPerms()
		noCreate.Create = false
		policy := testExtractPolicy(t, destDir, sourceName, username, noCreate)
		if err := extractTarGz(archivePath, destDir, policy); err == nil {
			t.Fatal("expected create permission error for symlink entry")
		}
	})
}
