package sql

import (
	"context"
	"errors"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

func TestSearchItemsContextReturnsCanceledContext(t *testing.T) {
	pop := pushTestIndexConfig(t, t.TempDir(), testIndexSQLConfig(settings.IndexStartupIntegrityOff))
	defer pop()

	db, _, err := NewIndexDB("search_context", "OFF", 1000, 32, true)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := db.CreateIndexTable(); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name  string
		query func(context.Context) error
	}{
		{
			name: "single source",
			query: func(ctx context.Context) error {
				rows, err := db.SearchItemsContext(ctx, "source", "/", false, nil, false, false)
				if rows != nil {
					rows.Close()
				}
				return err
			},
		},
		{
			name: "multiple sources",
			query: func(ctx context.Context) error {
				rows, err := db.SearchItemsMultiSourceContext(ctx, []string{"source"}, map[string]string{"source": "/"}, false, nil, false, false)
				if rows != nil {
					rows.Close()
				}
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			if err := tc.query(ctx); !errors.Is(err, context.Canceled) {
				t.Fatalf("query error = %v, want %v", err, context.Canceled)
			}
		})
	}
}
