package web

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
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
	lockWebGlobals(t)
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
	perms := fullPerms()
	user := &users.User{
		ID:           1,
		FrontendUser: users.FrontendUser{Username: username},
		BackendScopes: []users.BackendScope{{
			Path:        sourceRoot,
			Scope:       "/",
			Permissions: perms,
		}},
		BackendSourcePermissions: map[string]users.SourceFilePermissions{
			sourceRoot: perms,
		},
	}
	if err := state.CreateUser(user, ""); err != nil {
		t.Fatal(err)
	}
	if err := state.AllowUser(sourceRoot, utils.IndexPathFromNormalized("/", true), username); err != nil {
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

func requireExtractAccessDenied(t *testing.T, err error, destReal string, wantEntryInMsg string) {
	t.Helper()
	if !errors.Is(err, errExtractAccessDenied) {
		t.Fatalf("expected access-rule denial, got %v", err)
	}
	if !strings.HasPrefix(err.Error(), "access denied: ") {
		t.Fatalf("unexpected denial message: %v", err)
	}
	if strings.Contains(err.Error(), destReal) {
		t.Fatalf("denial error leaked real path: %v", err)
	}
	if wantEntryInMsg != "" && !strings.Contains(err.Error(), wantEntryInMsg) {
		t.Fatalf("expected archive entry %q in %v", wantEntryInMsg, err)
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

	t.Run("fails when parent path denied by access rules without writing", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/blocked", true), username); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "parent.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			w, err := zw.Create("allowed.txt")
			if err != nil {
				return err
			}
			if _, err = w.Write([]byte("ok")); err != nil {
				return err
			}
			w, err = zw.Create("blocked/child.txt")
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("nope"))
			return err
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		err := extractZipWithEncoding(zipPath, destDir, "", policy)
		requireExtractAccessDenied(t, err, destDir, "blocked/child.txt")
		if _, statErr := os.Stat(filepath.Join(destDir, "allowed.txt")); !os.IsNotExist(statErr) {
			t.Fatal("preflight should prevent partial extraction")
		}
		if _, statErr := os.Stat(filepath.Join(destDir, "blocked", "child.txt")); !os.IsNotExist(statErr) {
			t.Fatal("expected denied path not written")
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

	t.Run("fails when entry resolves through denied symlink target", func(t *testing.T) {
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
		requireExtractAccessDenied(t, extractZipWithEncoding(zipPath, destDir, "", policy), destDir, "alias/secret.txt")
		if _, err := os.Stat(filepath.Join(destDir, "alias", "secret.txt")); !os.IsNotExist(err) {
			t.Fatal("expected entry under denied target not written")
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

	t.Run("skips denied symlink entry at link path", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		if err := os.WriteFile(filepath.Join(destDir, "allowed.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkName := "secret-link"
		linkPath := filepath.Join(destDir, linkName)
		if err := os.Symlink("allowed.txt", linkPath); err != nil {
			t.Fatal(err)
		}
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/"+linkName, true), username); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "replace-link.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			hdr := &zip.FileHeader{Name: linkName, Method: zip.Store}
			hdr.SetMode(0o777 | os.ModeSymlink)
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("other.txt"))
			return err
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		requireExtractAccessDenied(t, extractZipWithEncoding(zipPath, destDir, "", policy), destDir, linkName)
		target, err := os.Readlink(linkPath)
		if err != nil || target != "allowed.txt" {
			t.Fatalf("symlink should be unchanged, target=%q err=%v", target, err)
		}
	})

	t.Run("fails mixed archive when any entry denied without partial write", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/hidden.txt", true), username); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "mixed.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			for _, name := range []string{"public.txt", "hidden.txt"} {
				w, err := zw.Create(name)
				if err != nil {
					return err
				}
				if _, err = w.Write([]byte("data")); err != nil {
					return err
				}
			}
			return nil
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		requireExtractAccessDenied(t, extractZipWithEncoding(zipPath, destDir, "", policy), destDir, "hidden.txt")
		if _, err := os.Stat(filepath.Join(destDir, "public.txt")); !os.IsNotExist(err) {
			t.Fatal("preflight should prevent partial extraction")
		}
		if _, err := os.Stat(filepath.Join(destDir, "hidden.txt")); !os.IsNotExist(err) {
			t.Fatal("expected hidden.txt not written")
		}
	})

	t.Run("dangling symlink requires modify", func(t *testing.T) {
		destDir, _, sourceName, username := setupExtractAccessPolicyTest(t)
		linkName := "dangling"
		if err := os.Symlink("missing-target", filepath.Join(destDir, linkName)); err != nil {
			t.Fatal(err)
		}
		zipPath := filepath.Join(t.TempDir(), "replace-dangling.zip")
		writeTestZip(t, zipPath, func(zw *zip.Writer) error {
			hdr := &zip.FileHeader{Name: linkName, Method: zip.Store}
			hdr.SetMode(0o777 | os.ModeSymlink)
			w, err := zw.CreateHeader(hdr)
			if err != nil {
				return err
			}
			_, err = w.Write([]byte("new-target"))
			return err
		})
		perms := fullPerms()
		perms.Modify = false
		policy := testExtractPolicy(t, destDir, sourceName, username, perms)
		if err := extractZipWithEncoding(zipPath, destDir, "", policy); err == nil {
			t.Fatal("expected modify permission error when replacing dangling symlink")
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

	t.Run("fails when parent path denied by access rules", func(t *testing.T) {
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
		requireExtractAccessDenied(t, extractTarGz(archivePath, destDir, policy), destDir, "blocked/child.txt")
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

	t.Run("skips denied symlink entry at link path", func(t *testing.T) {
		destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
		if err := os.WriteFile(filepath.Join(destDir, "allowed.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		linkName := "secret-link"
		if err := os.Symlink("allowed.txt", filepath.Join(destDir, linkName)); err != nil {
			t.Fatal(err)
		}
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/"+linkName, true), username); err != nil {
			t.Fatal(err)
		}
		archivePath := filepath.Join(t.TempDir(), "replace-link.tar.gz")
		writeTestTarGz(t, archivePath, func(tw *tar.Writer) error {
			return tw.WriteHeader(&tar.Header{
				Name:     linkName,
				Mode:     0o777,
				Typeflag: tar.TypeSymlink,
				Linkname: "other.txt",
			})
		})
		policy := testExtractPolicy(t, destDir, sourceName, username, fullPerms())
		requireExtractAccessDenied(t, extractTarGz(archivePath, destDir, policy), destDir, linkName)
	})
}

func TestUnarchiveHandler_accessDeniedAndSanitizesErrors(t *testing.T) {
	destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
	user, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/hidden.txt", true), username); err != nil {
		t.Fatal(err)
	}

	archivePath := filepath.Join(sourceRoot, "bundle.zip")
	writeTestZip(t, archivePath, func(zw *zip.Writer) error {
		for _, name := range []string{"visible.txt", "hidden.txt"} {
			w, err := zw.Create(name)
			if err != nil {
				return err
			}
			if _, err = w.Write([]byte("x")); err != nil {
				return err
			}
		}
		return nil
	})

	body := `{"fromSource":"data","toSource":"data","path":"/bundle.zip","destination":"/dest"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, handlerErr := unarchiveHandler(rec, req, &Context{User: &user})
	if status != http.StatusForbidden || handlerErr == nil {
		t.Fatalf("status=%d err=%v body=%s", status, handlerErr, rec.Body.String())
	}
	if !strings.Contains(handlerErr.Error(), "access denied: hidden.txt") {
		t.Fatalf("unexpected error: %v", handlerErr)
	}
	if strings.Contains(handlerErr.Error(), destDir) || strings.Contains(handlerErr.Error(), sourceRoot) {
		t.Fatalf("client error leaked real path: %v", handlerErr)
	}
	if _, err := os.Stat(filepath.Join(destDir, "visible.txt")); !os.IsNotExist(err) {
		t.Fatal("preflight should prevent partial extraction")
	}
	if _, err := os.Stat(filepath.Join(destDir, "hidden.txt")); !os.IsNotExist(err) {
		t.Fatal("expected hidden.txt not written")
	}

	// Permission failure must not expose the real destination path in the client error.
	existing := filepath.Join(destDir, "locked.txt")
	if err := os.WriteFile(existing, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	overwriteZip := filepath.Join(sourceRoot, "overwrite.zip")
	writeTestZip(t, overwriteZip, func(zw *zip.Writer) error {
		w, err := zw.Create("locked.txt")
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("new"))
		return err
	})
	noModify := fullPerms()
	noModify.Modify = false
	user.BackendSourcePermissions[sourceRoot] = noModify
	for i := range user.BackendScopes {
		if user.BackendScopes[i].Path == sourceRoot {
			user.BackendScopes[i].Permissions = noModify
		}
	}
	body = `{"fromSource":"data","toSource":"data","path":"/overwrite.zip","destination":"/dest"}`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, handlerErr = unarchiveHandler(rec, req, &Context{User: &user})
	if status != http.StatusForbidden || handlerErr == nil {
		t.Fatalf("status=%d err=%v", status, handlerErr)
	}
	if strings.Contains(handlerErr.Error(), destDir) || strings.Contains(handlerErr.Error(), sourceRoot) {
		t.Fatalf("client error leaked real path: %v", handlerErr)
	}
	if !strings.Contains(handlerErr.Error(), "modify permission required") {
		t.Fatalf("unexpected client error: %v", handlerErr)
	}
	_ = sourceName
}

func TestUnarchiveHandler_NestedScopePerEntryPermissions(t *testing.T) {
	destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
	user, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}
	readOnly := users.SourceFilePermissions{View: true, Download: true}
	user.BackendScopes = []users.BackendScope{
		{Path: sourceRoot, Scope: "/", Permissions: fullPerms()},
		{Path: sourceRoot, Scope: "/dest/protected", Permissions: readOnly},
	}

	archivePath := filepath.Join(sourceRoot, "nested.zip")
	writeTestZip(t, archivePath, func(zw *zip.Writer) error {
		w, err := zw.Create("protected/a.txt")
		if err != nil {
			return err
		}
		_, err = w.Write([]byte("blocked"))
		return err
	})

	body := `{"fromSource":"data","toSource":"data","path":"/nested.zip","destination":"/dest"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
	status, handlerErr := unarchiveHandler(rec, req, &Context{User: &user})
	if status != http.StatusForbidden || handlerErr == nil {
		t.Fatalf("expected 403 when extracting into read-only nested scope, got status=%d err=%v", status, handlerErr)
	}
	if _, statErr := os.Stat(filepath.Join(destDir, "protected", "a.txt")); !os.IsNotExist(statErr) {
		t.Fatal("protected/a.txt should not have been extracted")
	}
	_ = sourceName
}

func TestArchiveCreateAndMutation_NestedScopes(t *testing.T) {
	destDir, sourceRoot, sourceName, username := setupExtractAccessPolicyTest(t)
	user, err := state.GetUserByUsername(username)
	if err != nil {
		t.Fatal(err)
	}

	// Create source tree: /src/public.txt and /src/secret/hidden.txt
	srcDir := filepath.Join(sourceRoot, "src")
	secretDir := filepath.Join(srcDir, "secret")
	if err = os.MkdirAll(secretDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(srcDir, "public.txt"), []byte("pub"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(secretDir, "hidden.txt"), []byte("sec"), 0o644); err != nil {
		t.Fatal(err)
	}

	createNoModify := users.SourceFilePermissions{View: true, Download: true, Create: true, Modify: false, Delete: true}
	noDownloadNoDelete := users.SourceFilePermissions{View: true, Download: false, Create: false, Modify: false, Delete: false}
	user.BackendScopes = []users.BackendScope{
		{Path: sourceRoot, Scope: "/", Permissions: fullPerms()},
		{Path: sourceRoot, Scope: "/dest", Permissions: createNoModify},
		{Path: sourceRoot, Scope: "/src/secret", Permissions: noDownloadNoDelete},
	}

	// 1. Overwriting existing archive in /dest (Create=true, Modify=false) must fail without truncating.
	existingZip := filepath.Join(destDir, "existing.zip")
	if err = os.WriteFile(existingZip, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"fromSource":"data","toSource":"data","paths":["/src/public.txt"],"destination":"/dest/existing.zip"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/resources/archive", strings.NewReader(body))
	status, handlerErr := archiveCreateHandler(rec, req, &Context{User: &user})
	if status != http.StatusForbidden || handlerErr == nil {
		t.Fatalf("expected 403 when overwriting archive without Modify, got status=%d err=%v", status, handlerErr)
	}
	if data, readErr := os.ReadFile(existingZip); readErr != nil || string(data) != "keep-me" {
		t.Fatalf("existing archive should remain untouched, got %q err=%v", string(data), readErr)
	}

	// 2. Creating new archive from /src must skip /src/secret (Download=false).
	newZip := filepath.Join(destDir, "new.zip")
	body = `{"fromSource":"data","toSource":"data","paths":["/src"],"destination":"/dest/new.zip"}`
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/resources/archive", strings.NewReader(body))
	status, handlerErr = archiveCreateHandler(rec, req, &Context{User: &user})
	if status != http.StatusOK || handlerErr != nil {
		t.Fatalf("expected 200 creating new archive, got status=%d err=%v", status, handlerErr)
	}
	zr, err := zip.OpenReader(newZip)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
		if strings.Contains(f.Name, "hidden.txt") {
			t.Fatalf("archive should not include denied nested scope file: %s", f.Name)
		}
	}
	if len(names) == 0 {
		t.Fatal("expected public.txt in archive")
	}

	// 3. Subtree checks block deleting, moving, or copying /src because /src/secret has Delete=false and Download=false.
	if delErr := subtreeDeletePermitted(&Context{User: &user}, sourceName, "/src"); delErr == nil {
		t.Fatal("expected subtreeDeletePermitted to block deleting parent of Delete=false child scope")
	}
	if moveErr := subtreeScopesPermitted(&Context{User: &user}, sourceName, "/src", "move", func(p users.SourceFilePermissions) bool {
		return p.Modify && p.Delete
	}); moveErr == nil {
		t.Fatal("expected subtreeScopesPermitted to block moving parent of non-modifiable/non-deletable child scope")
	}
	if copyErr := subtreeScopesPermitted(&Context{User: &user}, sourceName, "/src", "copy", func(p users.SourceFilePermissions) bool {
		return p.Download
	}); copyErr == nil {
		t.Fatal("expected subtreeScopesPermitted to block copying parent of Download=false child scope")
	}

	// 4. Symlink inside writable /src pointing to read-only /src/secret must be blocked by mutationPermsAtRealPath.
	idx := indexing.GetIndex(sourceName)
	linkPath := filepath.Join(srcDir, "link_to_secret")
	if err = os.Symlink(secretDir, linkPath); err != nil {
		t.Fatal(err)
	}
	realTarget, err := idx.GetRealPathForMutation("/", "/src/link_to_secret/pwn.txt")
	if err != nil {
		t.Fatalf("GetRealPathForMutation: %v", err)
	}
	if mutErr := mutationPermsAtRealPath(&Context{User: &user}, idx, sourceName, realTarget, false, func(p users.SourceFilePermissions) bool {
		return p.Create
	}); mutErr == nil {
		t.Fatal("expected mutationPermsAtRealPath to reject symlink write targeting read-only nested scope")
	}

	// 5. ServeSingleFile rejects viewing a symlink inside viewable /src that resolves into a View=false child scope.
	user.BackendScopes[2].Permissions.View = false
	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/resources/view", nil)
	status, handlerErr = ServeSingleFile(rec, req, &Context{User: &user}, sourceName, "/src/link_to_secret/hidden.txt", "hidden.txt", ServeSingleFileOptions{ForceInline: true})
	if status != http.StatusForbidden || handlerErr == nil {
		t.Fatalf("expected ServeSingleFile to reject symlink targeting View=false scope, got status=%d err=%v", status, handlerErr)
	}
}
