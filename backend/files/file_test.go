package files

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func Test_GetRealPath(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		return
	}
	trimPrefix := filepath.Dir(filepath.Dir(cwd)) + "/"
	tests := []struct {
		name  string
		paths []string
		want  struct {
			path  string
			isDir bool
		}
	}{
		{
			name: "current directory",
			paths: []string{
				"./",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "backend/files",
				isDir: true,
			},
		},
		{
			name: "current directory",
			paths: []string{
				"./file.go",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "backend/files/file.go",
				isDir: false,
			},
		},
		{
			name: "other test case",
			paths: []string{
				"/mnt/storage",
			},
			want: struct {
				path  string
				isDir bool
			}{
				path:  "/mnt/storage",
				isDir: true,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			realPath, isDir, err := GetRealPath(tt.paths...)
			adjustedRealPath := strings.TrimPrefix(realPath, trimPrefix)
			if tt.want.path != adjustedRealPath || tt.want.isDir != isDir {
				t.Errorf("expected %v:%v but got: %v:%v", tt.want.path, tt.want.isDir, adjustedRealPath, isDir)
			}
			if err != nil {
				t.Error("got error", err)
			}
		})
	}
}
