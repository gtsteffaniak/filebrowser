package web

import (
	stderrors "errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/share"
	"github.com/gtsteffaniak/filebrowser/backend/internal/errors"
)

func TestPublicDownloadFileListForSingleFileShareUsesSharePath(t *testing.T) {
	t.Parallel()
	sourceRoot := t.TempDir()
	filePath := filepath.Join(sourceRoot, "workspace", "direct-test.txt")
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		t.Fatalf("create source directory: %v", err)
	}
	if err := os.WriteFile(filePath, []byte("direct download"), 0o644); err != nil {
		t.Fatalf("create source file: %v", err)
	}
	d := &Context{
		Share: share.Share{
			ShareColumns: share.ShareColumns{Path: "/workspace/direct-test.txt"},
			SourcePath:   sourceRoot,
		},
	}

	for _, files := range [][]string{{"direct-test.txt"}, {""}, nil} {
		got, err := publicDownloadFileList(d, files)
		if err != nil {
			t.Fatalf("publicDownloadFileList(%#v): %v", files, err)
		}
		want := []string{"/workspace/direct-test.txt"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("publicDownloadFileList(%#v) = %#v, want %#v", files, got, want)
		}
	}
}

func TestPublicDownloadFileListForDirectoryShareJoinsRelativePath(t *testing.T) {
	t.Parallel()
	d := &Context{
		Share: share.Share{
			ShareColumns: share.ShareColumns{Path: "/workspace"},
		},
	}

	got, err := publicDownloadFileList(d, []string{"nested/file.txt"})
	if err != nil {
		t.Fatalf("publicDownloadFileList: %v", err)
	}
	want := []string{"/workspace/nested/file.txt"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("publicDownloadFileList() = %#v, want %#v", got, want)
	}
}

func TestPublicDownloadFileListRejectsTraversal(t *testing.T) {
	t.Parallel()
	d := &Context{Share: share.Share{ShareColumns: share.ShareColumns{Path: "/workspace"}}}

	if _, err := publicDownloadFileList(d, []string{"../secret.txt"}); err == nil {
		t.Fatal("publicDownloadFileList() accepted a traversal path")
	}
}

func TestResolveDownloadInlineDisposition(t *testing.T) {
	t.Parallel()
	force, err := resolveDownloadInlineDisposition("readme.txt", true)
	if err != nil || !force {
		t.Fatalf("text inline: force=%v err=%v", force, err)
	}
	force, err = resolveDownloadInlineDisposition("clip.mp4", false)
	if err != nil || force {
		t.Fatalf("video download: force=%v err=%v", force, err)
	}
	_, err = resolveDownloadInlineDisposition("clip.mp4", true)
	if !stderrors.Is(err, errors.ErrUseMediaStream) {
		t.Fatalf("expected ErrUseMediaStream for inline video, got %v", err)
	}
}

func TestDownloadResponseRecordsActivity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		err    error
		want   bool
	}{
		{name: "200 ok", status: http.StatusOK, want: true},
		{name: "206 partial", status: http.StatusPartialContent, want: true},
		{name: "416 range unsatisfiable", status: http.StatusRequestedRangeNotSatisfiable, want: false},
		{name: "403 forbidden", status: http.StatusForbidden, want: false},
		{name: "error with 200", status: http.StatusOK, err: stderrors.New("write failed"), want: false},
		{name: "zero status", status: 0, want: false},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := downloadResponseRecordsActivity(tc.status, tc.err); got != tc.want {
				t.Fatalf("downloadResponseRecordsActivity(%d, err=%v) = %v, want %v", tc.status, tc.err != nil, got, tc.want)
			}
		})
	}
}
