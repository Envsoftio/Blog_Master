CREATE INDEX idx_review_comments_content
ON review_comments(project_id, content_id, id)
