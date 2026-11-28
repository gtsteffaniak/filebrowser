package preview

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/stretchr/testify/require"
)

// Let both callers observe the cold cache before either can generate a preview.
type concurrentPreviewCache struct {
	mu        sync.Mutex
	firstPair chan struct{}
	loads     int
	writes    int
	data      []byte
}

func (c *concurrentPreviewCache) Load(ctx context.Context, _ string) ([]byte, bool, error) {
	c.mu.Lock()
	data := c.data
	c.loads++
	if c.loads == 2 {
		close(c.firstPair)
	}
	c.mu.Unlock()

	select {
	case <-c.firstPair:
		return data, data != nil, nil
	case <-ctx.Done():
		return nil, false, ctx.Err()
	}
}

func (c *concurrentPreviewCache) Store(_ context.Context, _ string, data []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes++
	c.data = data
	return nil
}

func (c *concurrentPreviewCache) Delete(context.Context, string) error { return nil }

func TestConcurrentVideoPreviewsGenerateOnce(t *testing.T) {
	fixture, err := filepath.Abs("../../../frontend/tests/playwright-files/myfolder/testdata/sample-640x480.mp4")
	require.NoError(t, err)
	stat, err := os.Stat(fixture)
	require.NoError(t, err)

	previousMedia := settings.CurrentMediaSettings()
	previousAvailability := settings.Env.FFmpegAvailable
	t.Cleanup(func() {
		settings.ApplyMediaSettings(previousMedia)
		settings.Env.FFmpegAvailable = previousAvailability
	})
	media := previousMedia
	media.AlwaysUseFfmpeg = false
	enabled := true
	media.Convert.VideoPreview = map[settings.VideoPreviewType]*bool{settings.MP4VideoPreview: &enabled}
	settings.ApplyMediaSettings(media)
	settings.Env.FFmpegAvailable = true

	cache := &concurrentPreviewCache{firstPair: make(chan struct{})}
	previousService := service
	service = &Service{fileCache: cache, imageSem: make(chan struct{}, 1)}
	t.Cleanup(func() { service = previousService })

	file := iteminfo.ExtendedFileInfo{RealPath: fixture}
	file.Name = stat.Name()
	file.Type = "video/mp4"
	file.Size = stat.Size()
	file.ModTime = stat.ModTime()
	file.HasPreview = true

	ctx, cancel := context.WithTimeout(t.Context(), 12*time.Second)
	defer cancel()
	type result struct {
		data []byte
		err  error
	}
	results := make(chan result, 2)
	for range 2 {
		go func() {
			data, err := GetPreviewForFile(ctx, file, "small", "", 10)
			results <- result{data: data, err: err}
		}()
	}
	first, second := <-results, <-results
	require.NoError(t, first.err)
	require.NoError(t, second.err)
	require.NotEmpty(t, first.data)
	require.Equal(t, first.data, second.data)
	require.Equal(t, 1, cache.writes)
}
