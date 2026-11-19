package search

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/store/storetest"
)

// BenchmarkLibraryQuery is what one ⌘K keystroke costs off the UI thread:
// the search with previews, then the version counts and the total, over
// 10,000 snippets (internal/ui's library does the same calls).
func BenchmarkLibraryQuery(b *testing.B) {
	dir, err := os.MkdirTemp("", "lastab-bench-")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })
	st, err := store.Open(filepath.Join(dir, "lastab.db"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { st.Close() })
	ctx := context.Background()
	if _, err := storetest.Seed(ctx, st, 10000, 20, 1); err != nil {
		b.Fatal(err)
	}
	week := storetest.Base.AddDate(0, 0, -7)
	for _, c := range []struct {
		name string
		q    string
		o    store.ListOptions
	}{
		{"empty", "", store.ListOptions{Limit: 300}},
		{"like/周会", "周会", store.ListOptions{Limit: 300}},
		{"like/absent", storetest.Absent, store.ListOptions{Limit: 300}},
		{"fts/the", "the", store.ListOptions{Limit: 300}},
		{"fts/数据库", "数据库", store.ListOptions{Limit: 300}},
		{"fts/--since", "--since", store.ListOptions{Limit: 300}},
		{"fts/the/7d", "the", store.ListOptions{Limit: 300, Since: week}},
	} {
		b.Run(c.name, func(b *testing.B) {
			s := Searcher{Store: st}
			for b.Loop() {
				res, err := s.Query(ctx, c.q, c.o)
				if err != nil {
					b.Fatal(err)
				}
				ids := make([]string, len(res))
				for i, r := range res {
					ids[i] = r.ID
				}
				if _, err := st.VersionCounts(ctx, ids); err != nil {
					b.Fatal(err)
				}
				if _, err := st.Count(ctx); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
