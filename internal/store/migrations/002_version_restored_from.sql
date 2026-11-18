-- The version a restore brought back, for "Restored 14:34" in the history.
-- Empty for every other version, including the text saved before a restore.
ALTER TABLE version ADD COLUMN restored_from TEXT NOT NULL DEFAULT '';
