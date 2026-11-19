-- Who named a snippet whose title_source is 'manual': '' for the user, or
-- the agent's client name (claude-code, ...) when an agent renamed it
-- through `lastab retitle`. Only the user's own name is final: an agent may
-- rename what an agent named, never what the user did.
ALTER TABLE snippet ADD COLUMN title_by TEXT NOT NULL DEFAULT '';
