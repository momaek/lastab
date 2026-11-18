// Package snippet is the domain logic of snippets and the workspace: what
// counts as blank, creating on first real content, saving, closing (which
// is not deleting), deleting, ordering, titles and versions. The UI calls
// it directly; it does not depend on MyGo.
package snippet

import (
	"context"
	"strings"
	"time"
	"unicode"

	"github.com/momaek/lastab/internal/history"
	"github.com/momaek/lastab/internal/store"
	"github.com/momaek/lastab/internal/title"
)

// Service wraps the store with the rules of snippets.
type Service struct {
	Store *store.Store
	hist  history.Recorder
}

// New returns a service over st.
func New(st *store.Store) *Service {
	return &Service{Store: st, hist: history.Recorder{Store: st}}
}

// IsBlank reports whether body has nothing but whitespace. A blank tab is
// never written to the database.
func IsBlank(body string) bool {
	return strings.IndexFunc(body, func(r rune) bool { return !unicode.IsSpace(r) }) < 0
}

// Workspace returns the open tabs and the active one, to restore at launch.
func (s *Service) Workspace(ctx context.Context) ([]store.Snippet, string, error) {
	return s.Store.Workspace(ctx)
}

// Create writes a snippet once it first has real content.
func (s *Service) Create(ctx context.Context, kind store.Kind, body string, position int) (store.Snippet, error) {
	return s.Store.CreateSnippet(ctx, kind, body, position)
}

// SaveResult is a successful save.
type SaveResult struct {
	Rev        int
	ModifiedAt time.Time
}

// SaveBody writes a body based on revision baseRev. A snippet emptied stays:
// it is not deleted.
func (s *Service) SaveBody(ctx context.Context, id, body string, baseRev int) (SaveResult, error) {
	rev, at, err := s.Store.SaveBody(ctx, id, body, baseRev)
	return SaveResult{rev, at}, err
}

// UpdateTitle makes the automatic title of a snippet from its body and
// stores it, unless the user named it. A text snippet keeps its first
// automatic title; a to-do list's follows its progress. It returns the
// title now stored.
func (s *Service) UpdateTitle(ctx context.Context, id string) (string, store.TitleSource, error) {
	sn, err := s.Store.Get(ctx, id)
	if err != nil {
		return "", "", err
	}
	if sn.TitleSource == store.TitleManual {
		return sn.Title, sn.TitleSource, nil
	}
	if sn.TitleSource == store.TitleAuto && sn.Kind == store.KindText {
		return sn.Title, sn.TitleSource, nil
	}
	t := title.Make(title.Kind(sn.Kind), sn.Body)
	if t == "" {
		return sn.Title, sn.TitleSource, nil
	}
	if err := s.Store.SetTitle(ctx, id, t, store.TitleAuto); err != nil {
		return "", "", err
	}
	return t, store.TitleAuto, nil
}

// Rename sets a title by hand; automatic titles no longer replace it. An
// empty title goes back to automatic.
func (s *Service) Rename(ctx context.Context, id, t string) error {
	t = strings.TrimSpace(t)
	if t == "" {
		if err := s.Store.SetTitle(ctx, id, "", store.TitlePending); err != nil {
			return err
		}
		_, _, err := s.UpdateTitle(ctx, id)
		return err
	}
	return s.Store.SetTitle(ctx, id, t, store.TitleManual)
}

// Snapshot records a version, skipping one identical to the last.
func (s *Service) Snapshot(ctx context.Context, id, body string, reason store.Reason) (bool, error) {
	if IsBlank(body) {
		return false, nil
	}
	return s.hist.Snapshot(ctx, id, body, reason)
}

// Open adds a snippet found in the list to the workspace.
func (s *Service) Open(ctx context.Context, id string) (store.Snippet, error) {
	if err := s.Store.OpenTab(ctx, id); err != nil {
		return store.Snippet{}, err
	}
	return s.Store.Get(ctx, id)
}

// Close takes a snippet out of the workspace, recording a version of body
// first. The snippet stays, findable in the list.
func (s *Service) Close(ctx context.Context, id, body string) error {
	if _, err := s.Snapshot(ctx, id, body, store.ReasonClose); err != nil {
		return err
	}
	return s.Store.CloseTab(ctx, id)
}

// Delete removes a snippet for good, with its versions.
func (s *Service) Delete(ctx context.Context, id string) error {
	return s.Store.Delete(ctx, id)
}

// Reorder writes the tab bar's order and active tab.
func (s *Service) Reorder(ctx context.Context, ids []string, active string) error {
	return s.Store.SetTabOrder(ctx, ids, active)
}
