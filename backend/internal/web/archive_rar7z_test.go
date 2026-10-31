package web

import (
	"archive/zip"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/state"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

func archiveFixture(name string) string {
	return filepath.Join("testdata", "archives", name)
}

func assertFileContent(t *testing.T, path, want string) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != want {
		t.Fatalf("%s content = %q, want %q", path, b, want)
	}
}

func assertSymlink(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.Readlink(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s -> %q, want %q", path, got, want)
	}
}

func assertNoEntries(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected %s to be empty, found %d entries", dir, len(entries))
	}
}

func TestExtractRar(t *testing.T) {
	t.Parallel()
	t.Run("rar4 files, dirs, and symlink", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		if _, err := extractRar(archiveFixture("rar.rar"), out, "", nil, nil); err != nil {
			t.Fatalf("extractRar: %v", err)
		}
		assertFileContent(t, filepath.Join(out, "test.txt"), "test text document\r\n")
		assertFileContent(t, filepath.Join(out, "testdir", "test.txt"), "test text document\r\n")
		assertSymlink(t, filepath.Join(out, "testlink"), "test.txt")
		if fi, err := os.Stat(filepath.Join(out, "testemptydir")); err != nil || !fi.IsDir() {
			t.Fatalf("testemptydir not extracted as directory: %v", err)
		}
	})
	t.Run("rar5 symlinks", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		if _, err := extractRar(archiveFixture("rar5_symlink.rar"), out, "", nil, nil); err != nil {
			t.Fatalf("extractRar: %v", err)
		}
		assertFileContent(t, filepath.Join(out, "file.txt"), "1234\n")
		assertSymlink(t, filepath.Join(out, "symlink.txt"), "file.txt")
		assertSymlink(t, filepath.Join(out, "dirlink"), "dir")
	})
	t.Run("rar5 hard link", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		if _, err := extractRar(archiveFixture("rar5_hardlink.rar"), out, "", nil, nil); err != nil {
			t.Fatalf("extractRar: %v", err)
		}
		assertFileContent(t, filepath.Join(out, "hardlink.txt"), "1234\n")
	})
	t.Run("rar5 solid", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		if _, err := extractRar(archiveFixture("rar5_solid.rar"), out, "", nil, nil); err != nil {
			t.Fatalf("extractRar: %v", err)
		}
		for name, size := range map[string]int64{"test.bin": 1200, "test1.bin": 4096, "test6.bin": 4096} {
			fi, err := os.Stat(filepath.Join(out, name))
			if err != nil {
				t.Fatal(err)
			}
			if fi.Size() != size {
				t.Fatalf("%s size = %d, want %d", name, fi.Size(), size)
			}
		}
	})
}

func TestExtractRar_multiVolume(t *testing.T) {
	t.Parallel()
	first := archiveFixture("rar5_multiarchive_solid.part01.rar")
	t.Run("reads every volume", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		volumes, err := extractRar(first, out, "", nil, nil)
		if err != nil {
			t.Fatalf("extractRar: %v", err)
		}
		if len(volumes) != 4 {
			t.Fatalf("volumes = %v, want 4 parts", volumes)
		}
		fi, err := os.Stat(filepath.Join(out, "elf-Linux-ARMv7-ls"))
		if err != nil {
			t.Fatal(err)
		}
		if fi.Size() != 90808 {
			t.Fatalf("spanning file size = %d, want 90808", fi.Size())
		}
	})
	t.Run("denied volume stops extraction", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		allowed := func(name string) bool { return !strings.Contains(name, "part03") }
		_, err := extractRar(first, out, "", nil, allowed)
		if !errors.Is(err, errArchiveVolumeDenied) {
			t.Fatalf("err = %v, want errArchiveVolumeDenied", err)
		}
	})
}

func TestExtractRar_password(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, fixture, password string
		want                    error
	}{
		{name: "rar5 encrypted headers, no password", fixture: "rar5_encrypted_filenames.rar", want: errArchivePasswordRequired},
		{name: "rar5 encrypted headers, wrong password", fixture: "rar5_encrypted_filenames.rar", password: "nope", want: errArchiveBadPassword},
		{name: "rar5 encrypted data, no password", fixture: "rar5_encrypted.rar", want: errArchivePasswordRequired},
		{name: "rar5 encrypted data, wrong password", fixture: "rar5_encrypted.rar", password: "nope", want: errArchiveBadPassword},
		{name: "rar4 encrypted data, no password", fixture: "rar4_encrypted.rar", want: errArchivePasswordRequired},
		{name: "rar4 encrypted data, wrong password", fixture: "rar4_encrypted.rar", password: "nope", want: errArchiveBadPassword},
		// d.txt in this fixture uses a second password, so the first one cannot open it.
		{name: "rar4 file with a different password", fixture: "rar4_encrypted.rar", password: "password", want: errArchiveBadPassword},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := extractRar(archiveFixture(tc.fixture), t.TempDir(), tc.password, nil, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
		})
	}

	t.Run("correct password", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		if _, err := extractRar(archiveFixture("rar5_encrypted_filenames.rar"), out, "password", nil, nil); err != nil {
			t.Fatalf("extractRar: %v", err)
		}
		assertFileContent(t, filepath.Join(out, "d.txt"), "This is from d.txt")
	})
	t.Run("wrong password leaves no partial file", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		_, _ = extractRar(archiveFixture("rar4_encrypted.rar"), out, "nope", nil, nil)
		if _, err := os.Stat(filepath.Join(out, "b.txt")); !os.IsNotExist(err) {
			t.Fatalf("b.txt should not exist after a failed decrypt: %v", err)
		}
	})
}

func TestExtract7z(t *testing.T) {
	t.Parallel()
	t.Run("files, dirs, and symlink", func(t *testing.T) {
		t.Parallel()
		out := t.TempDir()
		if err := extract7z(archiveFixture("basic.7z"), out, "", nil); err != nil {
			t.Fatalf("extract7z: %v", err)
		}
		assertFileContent(t, filepath.Join(out, "dir", "sub", "file.txt"), "hello from 7z\n")
		assertFileContent(t, filepath.Join(out, "top.txt"), "top\n")
		assertSymlink(t, filepath.Join(out, "link.txt"), "top.txt")
	})
	t.Run("rejects path traversal before writing", func(t *testing.T) {
		t.Parallel()
		parent := t.TempDir()
		out := filepath.Join(parent, "out")
		if err := os.Mkdir(out, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := extract7z(archiveFixture("traversal.7z"), out, "", nil); err == nil {
			t.Fatal("expected traversal entry to be rejected")
		}
		if _, err := os.Stat(filepath.Join(parent, "evil.txt")); !os.IsNotExist(err) {
			t.Fatalf("evil.txt escaped the destination: %v", err)
		}
		assertNoEntries(t, out)
	})
}

func TestExtract7z_password(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, fixture, password string
		want                    error
	}{
		{name: "encrypted data, no password", fixture: "encrypted.7z", want: errArchivePasswordRequired},
		{name: "encrypted data, wrong password", fixture: "encrypted.7z", password: "nope", want: errArchiveBadPassword},
		{name: "encrypted headers, no password", fixture: "encrypted_headers.7z", want: errArchivePasswordRequired},
		{name: "encrypted headers, wrong password", fixture: "encrypted_headers.7z", password: "nope", want: errArchiveBadPassword},
		// Stored (uncompressed) entries decrypt to garbage; only the CRC catches it.
		{name: "encrypted store, no password", fixture: "encrypted_store.7z", want: errArchiveBadPassword},
		{name: "encrypted store, wrong password", fixture: "encrypted_store.7z", password: "nope", want: errArchiveBadPassword},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			out := t.TempDir()
			err := extract7z(archiveFixture(tc.fixture), out, tc.password, nil)
			if !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			if _, statErr := os.Stat(filepath.Join(out, "top.txt")); !os.IsNotExist(statErr) {
				t.Fatalf("top.txt should not exist after a failed decrypt: %v", statErr)
			}
		})
	}

	for _, fixture := range []string{"encrypted.7z", "encrypted_headers.7z", "encrypted_store.7z"} {
		t.Run("correct password "+fixture, func(t *testing.T) {
			t.Parallel()
			out := t.TempDir()
			if err := extract7z(archiveFixture(fixture), out, "password", nil); err != nil {
				t.Fatalf("extract7z: %v", err)
			}
			assertFileContent(t, filepath.Join(out, "top.txt"), "top\n")
		})
	}
}

func copyArchiveFixture(t *testing.T, name, dir string) {
	t.Helper()
	b, err := os.ReadFile(archiveFixture(name))
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestUnarchiveHandler_rar7z(t *testing.T) {
	unarchive := func(t *testing.T, username, body string) (int, error) {
		t.Helper()
		user, err := state.GetUserByUsername(username)
		if err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/api/resources/unarchive", strings.NewReader(body))
		return unarchiveHandler(httptest.NewRecorder(), req, &Context{User: &user})
	}

	t.Run("password required, then extracted with password", func(t *testing.T) {
		destDir, sourceRoot, _, username := setupExtractAccessPolicyTest(t)
		copyArchiveFixture(t, "rar5_encrypted_filenames.rar", sourceRoot)
		body := `{"fromSource":"data","path":"/rar5_encrypted_filenames.rar","destination":"/dest"}`
		status, err := unarchive(t, username, body)
		if status != http.StatusBadRequest || !errors.Is(err, errArchivePasswordRequired) {
			t.Fatalf("status=%d err=%v", status, err)
		}
		body = `{"fromSource":"data","path":"/rar5_encrypted_filenames.rar","destination":"/dest","password":"password"}`
		if status, err = unarchive(t, username, body); status != http.StatusOK {
			t.Fatalf("status=%d err=%v", status, err)
		}
		assertFileContent(t, filepath.Join(destDir, "d.txt"), "This is from d.txt")
	})

	t.Run("wrong 7z password is a client error", func(t *testing.T) {
		_, sourceRoot, _, username := setupExtractAccessPolicyTest(t)
		copyArchiveFixture(t, "encrypted_headers.7z", sourceRoot)
		body := `{"fromSource":"data","path":"/encrypted_headers.7z","destination":"/dest","password":"nope"}`
		status, err := unarchive(t, username, body)
		if status != http.StatusBadRequest || !errors.Is(err, errArchiveBadPassword) {
			t.Fatalf("status=%d err=%v", status, err)
		}
	})

	t.Run("password rejected for zip", func(t *testing.T) {
		_, sourceRoot, _, username := setupExtractAccessPolicyTest(t)
		writeTestZip(t, filepath.Join(sourceRoot, "plain.zip"), func(zw *zip.Writer) error {
			_, err := zw.Create("a.txt")
			return err
		})
		body := `{"fromSource":"data","path":"/plain.zip","destination":"/dest","password":"x"}`
		if status, _ := unarchive(t, username, body); status != http.StatusBadRequest {
			t.Fatalf("status=%d, want 400", status)
		}
	})

	t.Run("7z entries go through access preflight", func(t *testing.T) {
		destDir, sourceRoot, _, username := setupExtractAccessPolicyTest(t)
		copyArchiveFixture(t, "basic.7z", sourceRoot)
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/dest/dir/sub", true), username); err != nil {
			t.Fatal(err)
		}
		body := `{"fromSource":"data","path":"/basic.7z","destination":"/dest"}`
		status, err := unarchive(t, username, body)
		if status != http.StatusForbidden || !errors.Is(err, errExtractAccessDenied) {
			t.Fatalf("status=%d err=%v", status, err)
		}
		assertNoEntries(t, destDir)
	})

	t.Run("multi-volume rar deletes every volume after extract", func(t *testing.T) {
		destDir, sourceRoot, _, username := setupExtractAccessPolicyTest(t)
		parts := []string{"01", "02", "03", "04"}
		for _, p := range parts {
			copyArchiveFixture(t, "rar5_multiarchive_solid.part"+p+".rar", sourceRoot)
		}
		body := `{"fromSource":"data","path":"/rar5_multiarchive_solid.part01.rar","destination":"/dest","deleteAfter":true}`
		if status, err := unarchive(t, username, body); status != http.StatusOK {
			t.Fatalf("status=%d err=%v", status, err)
		}
		if _, err := os.Stat(filepath.Join(destDir, "elf-Linux-ARMv7-ls")); err != nil {
			t.Fatal(err)
		}
		for _, p := range parts {
			if _, err := os.Stat(filepath.Join(sourceRoot, "rar5_multiarchive_solid.part"+p+".rar")); !os.IsNotExist(err) {
				t.Fatalf("volume part%s not deleted: %v", p, err)
			}
		}
	})

	t.Run("multi-volume rar with a denied volume", func(t *testing.T) {
		_, sourceRoot, _, username := setupExtractAccessPolicyTest(t)
		for _, p := range []string{"01", "02", "03", "04"} {
			copyArchiveFixture(t, "rar5_multiarchive_solid.part"+p+".rar", sourceRoot)
		}
		if err := state.DenyUser(sourceRoot, utils.IndexPathFromNormalized("/rar5_multiarchive_solid.part03.rar", true), username); err != nil {
			t.Fatal(err)
		}
		body := `{"fromSource":"data","path":"/rar5_multiarchive_solid.part01.rar","destination":"/dest","deleteAfter":true}`
		status, err := unarchive(t, username, body)
		if status != http.StatusForbidden || !errors.Is(err, errArchiveVolumeDenied) {
			t.Fatalf("status=%d err=%v", status, err)
		}
		if _, err := os.Stat(filepath.Join(sourceRoot, "rar5_multiarchive_solid.part01.rar")); err != nil {
			t.Fatalf("first volume must survive a failed extract: %v", err)
		}
	})
}
