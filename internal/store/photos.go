package store

import (
	"context"
	"database/sql"
)

// CreatePhoto stores a picture.
func (s *Store) CreatePhoto(ctx context.Context, p *Photo) error {
	p.Bytes = len(p.Data)
	res, err := s.DB.ExecContext(ctx, `INSERT INTO photos (user_id, kind, mime, data, thumb, width, height, bytes) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		p.UserID, p.Kind, p.Mime, p.Data, p.Thumb, p.Width, p.Height, p.Bytes)
	if err != nil {
		return err
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// GetPhoto returns one of a person's pictures (thumb = the small one, when there is one).
func (s *Store) GetPhoto(ctx context.Context, uid, id int64, thumb bool) (*Photo, error) {
	var p Photo
	var t []byte
	err := s.DB.QueryRowContext(ctx, `SELECT id, user_id, kind, mime, data, thumb, width, height, bytes, created_at FROM photos WHERE user_id = ? AND id = ?`, uid, id).
		Scan(&p.ID, &p.UserID, &p.Kind, &p.Mime, &p.Data, &t, &p.Width, &p.Height, &p.Bytes, &p.CreatedAt)
	if err != nil {
		return nil, notFound(err)
	}
	if thumb && len(t) > 0 {
		p.Data = t
		p.Mime = "image/jpeg"
	}
	return &p, nil
}

// OwnsPhoto reports whether the picture is the person's.
func (s *Store) OwnsPhoto(ctx context.Context, uid, id int64) bool {
	var n int
	_ = s.DB.QueryRowContext(ctx, `SELECT COUNT(1) FROM photos WHERE user_id = ? AND id = ?`, uid, id).Scan(&n)
	return n > 0
}

// photoUsers: pictures something uses — a puzzle's, a stage's, an
// adventure's, or one inside a puzzle (a jigsaw, a quiz question) by its id.
const photoUsers = `SELECT photo_id FROM puzzles WHERE photo_id IS NOT NULL UNION SELECT photo_id FROM stages WHERE photo_id IS NOT NULL
	UNION SELECT photo_id FROM adventures WHERE photo_id IS NOT NULL
	UNION SELECT CAST(json_extract(data, '$.photo_id') AS INTEGER) FROM puzzles WHERE kind = 'jigsaw'
	UNION SELECT CAST(json_extract(q.value, '$.photo_id') AS INTEGER) FROM puzzles, json_each(puzzles.data, '$.questions') q WHERE kind = 'quiz'`

// PurgePhotos removes pictures uploaded more than a day ago that nothing uses.
func (s *Store) PurgePhotos(ctx context.Context) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM photos WHERE created_at < strftime('%Y-%m-%dT%H:%M:%fZ','now','-1 day') AND id NOT IN (`+photoUsers+`)`)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// PhotoBytes is the room a person's pictures take.
func (s *Store) PhotoBytes(ctx context.Context, uid int64) int64 {
	var n sql.NullInt64
	_ = s.DB.QueryRowContext(ctx, `SELECT SUM(bytes + COALESCE(length(thumb), 0)) FROM photos WHERE user_id = ?`, uid).Scan(&n)
	return n.Int64
}

// ListPhotoIDs lists a person's pictures in use (for the export).
func (s *Store) ListPhotoIDs(ctx context.Context, uid int64) ([]int64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT id FROM photos WHERE user_id = ? AND id IN (`+photoUsers+`) ORDER BY id`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func nullInt(n sql.NullInt64) *int64 {
	if !n.Valid {
		return nil
	}
	v := n.Int64
	return &v
}
