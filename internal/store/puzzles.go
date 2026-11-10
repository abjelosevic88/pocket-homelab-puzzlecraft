package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"
)

const puzzleCols = `p.id, p.user_id, p.kind, p.title, p.instructions, p.notes, p.level, p.language, p.tags, p.data, p.size_text, p.ready, p.problems,
	p.tested_at, p.photo_id, p.share_token, p.share_on, p.board, p.reveal, p.timer, p.source, p.source_ref, p.pinned, p.archived, p.created_at, p.updated_at,
	(SELECT COUNT(1) FROM plays x WHERE x.user_id = p.user_id AND x.what = 'puzzle' AND x.ref_id = p.id),
	(SELECT COUNT(1) FROM plays x WHERE x.user_id = p.user_id AND x.what = 'puzzle' AND x.ref_id = p.id AND x.finished_at IS NOT NULL),
	COALESCE((SELECT MIN(x.ms) FROM plays x WHERE x.user_id = p.user_id AND x.what = 'puzzle' AND x.ref_id = p.id AND x.finished_at IS NOT NULL AND x.ms > 0), 0)`

type scanner interface{ Scan(...any) error }

func scanPuzzle(sc scanner, withData bool) (*Puzzle, error) {
	var p Puzzle
	var tags, data string
	var tested sql.NullString
	var photo sql.NullInt64
	var ready, share, board, reveal, timer, pinned, archived int
	if err := sc.Scan(&p.ID, &p.UserID, &p.Kind, &p.Title, &p.Instructions, &p.Notes, &p.Level, &p.Language, &tags, &data, &p.SizeText, &ready, &p.Problems,
		&tested, &photo, &p.ShareToken, &share, &board, &reveal, &timer, &p.Source, &p.SourceRef, &pinned, &archived, &p.CreatedAt, &p.UpdatedAt,
		&p.Plays, &p.Solves, &p.BestMs); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(tags), &p.Tags)
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if withData {
		p.Data = json.RawMessage(data)
	}
	if tested.Valid {
		p.TestedAt = &tested.String
	}
	p.PhotoID = nullInt(photo)
	p.Ready, p.ShareOn, p.Board, p.Reveal, p.Timer, p.Pinned, p.Archived = ready == 1, share == 1, board == 1, reveal == 1, timer == 1, pinned == 1, archived == 1
	return &p, nil
}

// PuzzleFilter narrows the list.
type PuzzleFilter struct {
	Kind     string
	Q        string
	Tag      string
	Archived bool
	Shared   bool
	Limit    int
	IDs      []int64
}

// ListPuzzles lists a person's puzzles, pinned and newest first (no data).
func (s *Store) ListPuzzles(ctx context.Context, uid int64, f PuzzleFilter) ([]Puzzle, error) {
	q := `SELECT ` + puzzleCols + ` FROM puzzles p WHERE p.user_id = ? AND p.archived = ?`
	args := []any{uid, boolInt(f.Archived)}
	if f.Kind != "" {
		q += ` AND p.kind = ?`
		args = append(args, f.Kind)
	}
	if f.Shared {
		q += ` AND p.share_on = 1`
	}
	if t := strings.TrimSpace(f.Q); t != "" {
		q += ` AND (p.title LIKE ? OR p.instructions LIKE ? OR p.notes LIKE ? OR p.tags LIKE ?)`
		like := "%" + t + "%"
		args = append(args, like, like, like, like)
	}
	if f.Tag != "" {
		q += ` AND EXISTS (SELECT 1 FROM json_each(p.tags) WHERE value = ?)`
		args = append(args, f.Tag)
	}
	if len(f.IDs) > 0 {
		q += ` AND p.id IN (` + strings.TrimSuffix(strings.Repeat("?,", len(f.IDs)), ",") + `)`
		for _, id := range f.IDs {
			args = append(args, id)
		}
	}
	q += ` ORDER BY p.pinned DESC, p.updated_at DESC`
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Puzzle{}
	for rows.Next() {
		p, err := scanPuzzle(rows, false)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// GetPuzzle returns one of a person's puzzles with its definition.
func (s *Store) GetPuzzle(ctx context.Context, uid, id int64) (*Puzzle, error) {
	p, err := scanPuzzle(s.DB.QueryRowContext(ctx, `SELECT `+puzzleCols+` FROM puzzles p WHERE p.user_id = ? AND p.id = ?`, uid, id), true)
	return p, notFound(err)
}

// PuzzleByShare finds a shared puzzle by its link (anyone's).
func (s *Store) PuzzleByShare(ctx context.Context, token string) (*Puzzle, error) {
	if len(token) < 16 {
		return nil, ErrNotFound
	}
	p, err := scanPuzzle(s.DB.QueryRowContext(ctx, `SELECT `+puzzleCols+` FROM puzzles p WHERE p.share_token = ? AND p.share_on = 1 AND p.archived = 0`, token), true)
	return p, notFound(err)
}

// CreatePuzzle adds a puzzle.
func (s *Store) CreatePuzzle(ctx context.Context, p *Puzzle) error {
	if p.Tags == nil {
		p.Tags = []string{}
	}
	if len(p.Data) == 0 {
		p.Data = json.RawMessage(`{}`)
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO puzzles (user_id, kind, title, instructions, notes, level, language, tags, data, size_text, ready, problems, tested_at,
		photo_id, share_token, share_on, board, reveal, timer, source, source_ref, pinned, archived, created_at, updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		p.UserID, p.Kind, p.Title, p.Instructions, p.Notes, p.Level, p.Language, toJSON(p.Tags), string(p.Data), p.SizeText, boolInt(p.Ready), p.Problems, p.TestedAt,
		nullI(p.PhotoID), p.ShareToken, boolInt(p.ShareOn), boolInt(p.Board), boolInt(p.Reveal), boolInt(p.Timer), p.Source, p.SourceRef, boolInt(p.Pinned), boolInt(p.Archived), Now(), Now())
	if err != nil {
		return conflict(err)
	}
	p.ID, _ = res.LastInsertId()
	return nil
}

// UpdatePuzzle saves a puzzle.
func (s *Store) UpdatePuzzle(ctx context.Context, p *Puzzle) error {
	if p.Tags == nil {
		p.Tags = []string{}
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE puzzles SET kind = ?, title = ?, instructions = ?, notes = ?, level = ?, language = ?, tags = ?, data = ?, size_text = ?, ready = ?,
		problems = ?, tested_at = ?, photo_id = ?, share_on = ?, board = ?, reveal = ?, timer = ?, source_ref = ?, pinned = ?, archived = ?, updated_at = ? WHERE user_id = ? AND id = ?`,
		p.Kind, p.Title, p.Instructions, p.Notes, p.Level, p.Language, toJSON(p.Tags), string(p.Data), p.SizeText, boolInt(p.Ready), p.Problems, p.TestedAt,
		nullI(p.PhotoID), boolInt(p.ShareOn), boolInt(p.Board), boolInt(p.Reveal), boolInt(p.Timer), p.SourceRef, boolInt(p.Pinned), boolInt(p.Archived), Now(), p.UserID, p.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// SetShareToken gives a puzzle a new link (the old one stops working).
func (s *Store) SetShareToken(ctx context.Context, table string, uid, id int64, token string) error {
	if table != "puzzles" && table != "books" && table != "adventures" {
		return ErrNotFound
	}
	_, err := s.DB.ExecContext(ctx, `UPDATE `+table+` SET share_token = ? WHERE user_id = ? AND id = ?`, token, uid, id)
	return err
}

// DeletePuzzle removes a puzzle and its plays.
func (s *Store) DeletePuzzle(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM puzzles WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM plays WHERE user_id = ? AND what = 'puzzle' AND ref_id = ?`, uid, id)
	return nil
}

// PuzzleBySourceRef finds what another app added before (to add it once).
func (s *Store) PuzzleBySourceRef(ctx context.Context, uid int64, source, ref string) (*Puzzle, error) {
	if ref == "" {
		return nil, ErrNotFound
	}
	p, err := scanPuzzle(s.DB.QueryRowContext(ctx, `SELECT `+puzzleCols+` FROM puzzles p WHERE p.user_id = ? AND p.source = ? AND p.source_ref = ?`, uid, source, ref), true)
	return p, notFound(err)
}

// PuzzleTags lists a person's tags with how often each is used.
func (s *Store) PuzzleTags(ctx context.Context, uid int64) (map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT j.value, COUNT(1) FROM puzzles p, json_each(p.tags) j WHERE p.user_id = ? AND p.archived = 0 GROUP BY j.value`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}

// KindCounts is how many puzzles a person has of each kind.
func (s *Store) KindCounts(ctx context.Context, uid int64) (map[string]int, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT kind, COUNT(1) FROM puzzles WHERE user_id = ? AND archived = 0 GROUP BY kind`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return nil, err
		}
		out[k] = n
	}
	return out, rows.Err()
}

// PuzzleUses says where a puzzle is used (books, adventure stages).
func (s *Store) PuzzleUses(ctx context.Context, uid, id int64) []string {
	rows, err := s.DB.QueryContext(ctx, `SELECT 'book', b.title FROM book_items i JOIN books b ON b.id = i.book_id WHERE i.user_id = ? AND i.puzzle_id = ?
		UNION ALL SELECT 'adventure', a.title FROM stages st JOIN adventures a ON a.id = st.adventure_id WHERE st.user_id = ? AND st.puzzle_id = ?`, uid, id, uid, id)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var k, t string
		if rows.Scan(&k, &t) == nil {
			out = append(out, t)
		}
	}
	return out
}
