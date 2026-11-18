-- Snippets: the body exactly as typed (line endings kept), title as list
-- metadata only, never written into the body.
CREATE TABLE snippet (
  id            TEXT PRIMARY KEY,
  kind          TEXT NOT NULL CHECK (kind IN ('text','todo')),
  body          TEXT NOT NULL,
  title         TEXT NOT NULL DEFAULT '',
  title_source  TEXT NOT NULL DEFAULT 'pending' CHECK (title_source IN ('pending','auto','manual')),
  created_at    INTEGER NOT NULL,
  modified_at   INTEGER NOT NULL,
  rev           INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX snippet_modified ON snippet(modified_at DESC);

-- Open tabs. The recent switcher only looks here.
CREATE TABLE workspace_tab (
  snippet_id  TEXT PRIMARY KEY REFERENCES snippet(id) ON DELETE CASCADE,
  position    INTEGER NOT NULL,
  opened_at   INTEGER NOT NULL,
  is_active   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE version (
  id            TEXT PRIMARY KEY,
  snippet_id    TEXT NOT NULL REFERENCES snippet(id) ON DELETE CASCADE,
  body          TEXT NOT NULL,
  created_at    INTEGER NOT NULL,
  reason        TEXT NOT NULL CHECK (reason IN ('paused','switch','close','copy','restore')),
  lines_changed INTEGER NOT NULL
);
CREATE INDEX version_snippet ON version(snippet_id, created_at DESC);

-- Full-text search over title and body. trigram gives substring matches for
-- Chinese, English and code symbols alike.
CREATE VIRTUAL TABLE snippet_fts USING fts5(
  title, body,
  content = 'snippet', content_rowid = 'rowid',
  tokenize = 'trigram case_sensitive 0'
);

CREATE TRIGGER snippet_fts_ai AFTER INSERT ON snippet BEGIN
  INSERT INTO snippet_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
END;
CREATE TRIGGER snippet_fts_ad AFTER DELETE ON snippet BEGIN
  INSERT INTO snippet_fts(snippet_fts, rowid, title, body) VALUES ('delete', old.rowid, old.title, old.body);
END;
CREATE TRIGGER snippet_fts_au AFTER UPDATE OF title, body ON snippet BEGIN
  INSERT INTO snippet_fts(snippet_fts, rowid, title, body) VALUES ('delete', old.rowid, old.title, old.body);
  INSERT INTO snippet_fts(rowid, title, body) VALUES (new.rowid, new.title, new.body);
END;

CREATE TABLE setting (
  key    TEXT PRIMARY KEY,
  value  TEXT NOT NULL
);
