// Package history records versions of snippets: when typing pauses, on
// switching away, closing, copying all, and before a restore, skipping a
// version identical to the one before.
package history

import (
	"context"
	"errors"

	"github.com/sergi/go-diff/diffmatchpatch"

	"github.com/momaek/lastab/internal/store"
)

// LinesChanged counts the lines added or removed between two texts, the
// "changed N lines" of the history list.
func LinesChanged(old, new string) int {
	dmp := diffmatchpatch.New()
	a, b, lines := dmp.DiffLinesToChars(old, new)
	diffs := dmp.DiffCharsToLines(dmp.DiffMain(a, b, false), lines)
	var added, removed int
	for _, d := range diffs {
		n := countLines(d.Text)
		switch d.Type {
		case diffmatchpatch.DiffInsert:
			added += n
		case diffmatchpatch.DiffDelete:
			removed += n
		}
	}
	// A line replaced counts once, as "changed".
	return max(added, removed)
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	if s[len(s)-1] != '\n' {
		n++
	}
	return n
}

// Recorder writes versions to a store.
type Recorder struct {
	Store *store.Store
}

// Snapshot records body as a version of the snippet, unless it equals the
// latest version. It reports whether it recorded one.
func (r Recorder) Snapshot(ctx context.Context, snippetID, body string, reason store.Reason) (bool, error) {
	return r.SnapshotFrom(ctx, snippetID, body, reason, "")
}

// SnapshotFrom is Snapshot recording the version a restore brought back.
func (r Recorder) SnapshotFrom(ctx context.Context, snippetID, body string, reason store.Reason, restoredFrom string) (bool, error) {
	prev := ""
	last, err := r.Store.LatestVersion(ctx, snippetID)
	switch {
	case err == nil:
		if last.Body == body {
			return false, nil
		}
		prev = last.Body
	case errors.Is(err, store.ErrNotFound):
	default:
		return false, err
	}
	_, err = r.Store.AddVersionFrom(ctx, snippetID, body, reason, LinesChanged(prev, body), restoredFrom)
	return err == nil, err
}
