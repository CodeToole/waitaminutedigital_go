ALTER TABLE highlight ADD COLUMN article_id INTEGER REFERENCES article(id) ON DELETE CASCADE;
CREATE UNIQUE INDEX IF NOT EXISTS highlight_article_id_idx ON highlight(article_id);