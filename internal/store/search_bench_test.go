package store_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/store/storetest"
)

// The benchmarks share one seeded database of benchSnippets snippets: the
// size the dev doc's validation item 7 asks about.
const benchSnippets = 10000

var bench struct {
	once   sync.Once
	dir    string
	store  *store.Store
	corpus storetest.Corpus
	err    error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if bench.store != nil {
		bench.store.Close()
	}
	if bench.dir != "" {
		os.RemoveAll(bench.dir)
	}
	os.Exit(code)
}

func benchStore(b *testing.B) (*store.Store, storetest.Corpus) {
	b.Helper()
	bench.once.Do(func() {
		bench.dir, bench.err = os.MkdirTemp("", "lastab-bench-")
		if bench.err != nil {
			return
		}
		bench.store, bench.err = store.Open(filepath.Join(bench.dir, "lastab.db"))
		if bench.err != nil {
			return
		}
		start := time.Now()
		bench.corpus, bench.err = storetest.Seed(context.Background(), bench.store, benchSnippets, 20, 1)
		b.Logf("seeded %d snippets in %v", benchSnippets, time.Since(start).Round(time.Millisecond))
	})
	if bench.err != nil {
		b.Fatal(bench.err)
	}
	return bench.store, bench.corpus
}

// The library's limit; internal/ui.LibraryLimit.
const libraryLimit = 300

func BenchmarkSearch(b *testing.B) {
	base := storetest.Base
	week, month := base.AddDate(0, 0, -7), base.AddDate(0, 0, -30)
	lim := store.ListOptions{Limit: libraryLimit}
	cases := []struct {
		name string
		q    string
		o    store.ListOptions
	}{
		// Under three characters: the LIKE fallback.
		{"like/周会", "周会", lim},
		{"like/周会/nolimit", "周会", store.ListOptions{}},
		{"like/发版", "发版", lim},
		{"like/{\"", `{"`, lim},
		{"like/absent", storetest.Absent, lim},
		{"like/周会/7d", "周会", store.ListOptions{Limit: libraryLimit, Since: week}},
		{"like/absent/30d", storetest.Absent, store.ListOptions{Limit: libraryLimit, Since: month}},
		{"like/发版/closed", "发版", store.ListOptions{Limit: libraryLimit, ClosedOnly: true}},
		{"like/发版/open", "发版", store.ListOptions{Limit: libraryLimit, OpenOnly: true}},
		// Three characters or more: the trigram index.
		{"fts/数据库", "数据库", lim},
		{"fts/--since", "--since", lim},
		{"fts/{\"id", `{"id`, lim},
		{"fts//v2/", "/v2/", lim},
		{"fts/Authorization", "Authorization", lim},
		{"fts/rare", "certificate", lim},
		{"fts/absent", "zebra crossing", lim},
		// Common terms that match thousands of snippets.
		{"fts/the", "the", lim},
		{"fts/the/nolimit", "the", store.ListOptions{}},
		{"fts/the/7d", "the", store.ListOptions{Limit: libraryLimit, Since: week}},
		{"fts/the/30d/closed", "the", store.ListOptions{Limit: libraryLimit, Since: month, ClosedOnly: true}},
		{"fts/the/open", "the", store.ListOptions{Limit: libraryLimit, OpenOnly: true}},
		{"fts/the/todo", "the", store.ListOptions{Limit: libraryLimit, Kind: store.KindTodo}},
		{"fts/数据库/7d", "数据库", store.ListOptions{Limit: libraryLimit, Since: week}},
	}
	ctx := context.Background()
	for _, c := range cases {
		b.Run(c.name, func(b *testing.B) {
			s, _ := benchStore(b)
			var n int
			for b.Loop() {
				res, err := s.Search(ctx, c.q, c.o)
				if err != nil {
					b.Fatal(err)
				}
				n = len(res)
			}
			b.ReportMetric(float64(n), "results")
		})
	}
}

func BenchmarkList(b *testing.B) {
	ctx := context.Background()
	for _, c := range []struct {
		name string
		o    store.ListOptions
	}{
		{"300", store.ListOptions{Limit: libraryLimit}},
		{"300/7d", store.ListOptions{Limit: libraryLimit, Since: storetest.Base.AddDate(0, 0, -7)}},
		{"300/closed", store.ListOptions{Limit: libraryLimit, ClosedOnly: true}},
		{"all", store.ListOptions{}},
	} {
		b.Run(c.name, func(b *testing.B) {
			s, _ := benchStore(b)
			var n int
			for b.Loop() {
				res, err := s.List(ctx, c.o)
				if err != nil {
					b.Fatal(err)
				}
				n = len(res)
			}
			b.ReportMetric(float64(n), "results")
		})
	}
}

func BenchmarkVersionCounts(b *testing.B) {
	ctx := context.Background()
	s, c := benchStore(b)
	ids := c.IDs[:libraryLimit]
	for b.Loop() {
		if _, err := s.VersionCounts(ctx, ids); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCount(b *testing.B) {
	ctx := context.Background()
	s, _ := benchStore(b)
	for b.Loop() {
		if _, err := s.Count(ctx); err != nil {
			b.Fatal(err)
		}
	}
}
