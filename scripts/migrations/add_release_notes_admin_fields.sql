ALTER TABLE release_notes
    ADD COLUMN IF NOT EXISTS title TEXT DEFAULT '',
    ADD COLUMN IF NOT EXISTS content TEXT DEFAULT '',
    ADD COLUMN IF NOT EXISTS content_format TEXT DEFAULT 'markdown',
    ADD COLUMN IF NOT EXISTS notes_format TEXT DEFAULT 'json',
    ADD COLUMN IF NOT EXISTS sanitized_content TEXT DEFAULT '',
    ADD COLUMN IF NOT EXISTS publish_state TEXT DEFAULT 'draft',
    ADD COLUMN IF NOT EXISTS published_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS image_refs TEXT DEFAULT '',
    ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ DEFAULT NOW();

CREATE INDEX IF NOT EXISTS idx_release_notes_version ON release_notes (version);
CREATE INDEX IF NOT EXISTS idx_release_notes_publish_state ON release_notes (publish_state);
CREATE INDEX IF NOT EXISTS idx_release_notes_published_at ON release_notes (published_at DESC);
CREATE INDEX IF NOT EXISTS idx_release_notes_notes_format ON release_notes (notes_format);
