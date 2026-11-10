package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

const advCols = `a.id, a.user_id, a.kind, a.title, a.intro, a.finale, a.theme, a.date, a.time, a.place, a.players, a.minutes, a.hint_penalty, a.status, a.notes, a.prep,
	a.share_token, a.share_on, a.board, a.calendar_ref, a.calendar_url, a.photo_id, a.source, a.created_at, a.updated_at,
	(SELECT COUNT(1) FROM stages s WHERE s.user_id = a.user_id AND s.adventure_id = a.id),
	(SELECT COUNT(1) FROM plays x WHERE x.user_id = a.user_id AND x.what = 'adventure' AND x.ref_id = a.id),
	(SELECT COUNT(1) FROM plays x WHERE x.user_id = a.user_id AND x.what = 'adventure' AND x.ref_id = a.id AND x.finished_at IS NULL
		AND x.last_at > strftime('%Y-%m-%dT%H:%M:%fZ','now','-15 minutes'))`

func scanAdventure(sc scanner) (*Adventure, error) {
	var a Adventure
	var prep string
	var share, board int
	var photo sql.NullInt64
	if err := sc.Scan(&a.ID, &a.UserID, &a.Kind, &a.Title, &a.Intro, &a.Finale, &a.Theme, &a.Date, &a.Time, &a.Place, &a.Players, &a.Minutes, &a.HintPenalty, &a.Status, &a.Notes, &prep,
		&a.ShareToken, &share, &board, &a.CalendarRef, &a.CalendarURL, &photo, &a.Source, &a.CreatedAt, &a.UpdatedAt, &a.StagesN, &a.Plays, &a.Live); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(prep), &a.Prep)
	if a.Prep == nil {
		a.Prep = []PrepItem{}
	}
	a.ShareOn, a.Board = share == 1, board == 1
	a.PhotoID = nullInt(photo)
	return &a, nil
}

// ListAdventures lists a person's escape rooms and hunts, coming up first.
func (s *Store) ListAdventures(ctx context.Context, uid int64) ([]Adventure, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+advCols+` FROM adventures a WHERE a.user_id = ?
		ORDER BY a.status = 'done', CASE WHEN a.date >= date('now') THEN 0 WHEN a.date = '' THEN 1 ELSE 2 END, a.date, a.updated_at DESC`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Adventure{}
	for rows.Next() {
		a, err := scanAdventure(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return out, rows.Err()
}

// GetAdventure returns an adventure with its stages.
func (s *Store) GetAdventure(ctx context.Context, uid, id int64) (*Adventure, error) {
	a, err := scanAdventure(s.DB.QueryRowContext(ctx, `SELECT `+advCols+` FROM adventures a WHERE a.user_id = ? AND a.id = ?`, uid, id))
	if err != nil {
		return nil, notFound(err)
	}
	a.Stages, err = s.ListStages(ctx, uid, id)
	return a, err
}

// AdventureByShare finds a shared adventure by its link.
func (s *Store) AdventureByShare(ctx context.Context, token string) (*Adventure, error) {
	if len(token) < 16 {
		return nil, ErrNotFound
	}
	a, err := scanAdventure(s.DB.QueryRowContext(ctx, `SELECT `+advCols+` FROM adventures a WHERE a.share_token = ? AND a.share_on = 1`, token))
	if err != nil {
		return nil, notFound(err)
	}
	a.Stages, err = s.ListStages(ctx, a.UserID, a.ID)
	return a, err
}

// SaveAdventure creates or updates an adventure (not its stages).
func (s *Store) SaveAdventure(ctx context.Context, a *Adventure) error {
	if a.Prep == nil {
		a.Prep = []PrepItem{}
	}
	if a.ID == 0 {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO adventures (user_id, kind, title, intro, finale, theme, date, time, place, players, minutes, hint_penalty, status, notes, prep,
			share_token, share_on, board, calendar_ref, calendar_url, photo_id, source) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			a.UserID, a.Kind, a.Title, a.Intro, a.Finale, a.Theme, a.Date, a.Time, a.Place, a.Players, a.Minutes, a.HintPenalty, a.Status, a.Notes, toJSON(a.Prep),
			a.ShareToken, boolInt(a.ShareOn), boolInt(a.Board), a.CalendarRef, a.CalendarURL, nullI(a.PhotoID), a.Source)
		if err != nil {
			return conflict(err)
		}
		a.ID, _ = res.LastInsertId()
		return nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE adventures SET kind = ?, title = ?, intro = ?, finale = ?, theme = ?, date = ?, time = ?, place = ?, players = ?, minutes = ?,
		hint_penalty = ?, status = ?, notes = ?, prep = ?, share_on = ?, board = ?, calendar_ref = ?, calendar_url = ?, photo_id = ?, updated_at = ? WHERE user_id = ? AND id = ?`,
		a.Kind, a.Title, a.Intro, a.Finale, a.Theme, a.Date, a.Time, a.Place, a.Players, a.Minutes, a.HintPenalty, a.Status, a.Notes, toJSON(a.Prep),
		boolInt(a.ShareOn), boolInt(a.Board), a.CalendarRef, a.CalendarURL, nullI(a.PhotoID), Now(), a.UserID, a.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteAdventure removes an adventure, its stages and plays.
func (s *Store) DeleteAdventure(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM adventures WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	_, _ = s.DB.ExecContext(ctx, `DELETE FROM plays WHERE user_id = ? AND what = 'adventure' AND ref_id = ?`, uid, id)
	return nil
}

const stageCols = `s.id, s.user_id, s.adventure_id, s.sort, s.title, s.story, s.photo_id, s.lock, s.puzzle_id, s.answers, s.lock_hint, s.hints, s.hidden,
	s.place_name, s.lat, s.lng, s.place_ref, s.unlock, s.props, s.code, s.created_at, COALESCE(p.title, ''), COALESCE(p.kind, '')`

func scanStage(sc scanner) (*Stage, error) {
	var st Stage
	var answers, hints string
	var photo, puzzle sql.NullInt64
	var lat, lng sql.NullFloat64
	if err := sc.Scan(&st.ID, &st.UserID, &st.AdventureID, &st.Sort, &st.Title, &st.Story, &photo, &st.Lock, &puzzle, &answers, &st.LockHint, &hints, &st.Hidden,
		&st.PlaceName, &lat, &lng, &st.PlaceRef, &st.Unlock, &st.Props, &st.Code, &st.CreatedAt, &st.PuzzleTitle, &st.PuzzleKind); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(answers), &st.Answers)
	_ = json.Unmarshal([]byte(hints), &st.Hints)
	if st.Answers == nil {
		st.Answers = []string{}
	}
	if st.Hints == nil {
		st.Hints = []Hint{}
	}
	st.PhotoID, st.PuzzleID = nullInt(photo), nullInt(puzzle)
	if lat.Valid && lng.Valid {
		st.Lat, st.Lng = &lat.Float64, &lng.Float64
	}
	return &st, nil
}

// ListStages lists an adventure's stages in order.
func (s *Store) ListStages(ctx context.Context, uid, advID int64) ([]Stage, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+stageCols+` FROM stages s LEFT JOIN puzzles p ON p.id = s.puzzle_id AND p.user_id = s.user_id
		WHERE s.user_id = ? AND s.adventure_id = ? ORDER BY s.sort, s.id`, uid, advID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Stage{}
	for rows.Next() {
		st, err := scanStage(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *st)
	}
	return out, rows.Err()
}

// GetStage returns one stage.
func (s *Store) GetStage(ctx context.Context, uid, id int64) (*Stage, error) {
	st, err := scanStage(s.DB.QueryRowContext(ctx, `SELECT `+stageCols+` FROM stages s LEFT JOIN puzzles p ON p.id = s.puzzle_id AND p.user_id = s.user_id
		WHERE s.user_id = ? AND s.id = ?`, uid, id))
	return st, notFound(err)
}

func nullF(f *float64) any {
	if f == nil {
		return nil
	}
	return *f
}

// SaveStage creates or updates a stage.
func (s *Store) SaveStage(ctx context.Context, st *Stage) error {
	if st.Answers == nil {
		st.Answers = []string{}
	}
	if st.Hints == nil {
		st.Hints = []Hint{}
	}
	if st.ID == 0 {
		if st.Sort == 0 {
			_ = s.DB.QueryRowContext(ctx, `SELECT COALESCE(MAX(sort), 0) + 1 FROM stages WHERE user_id = ? AND adventure_id = ?`, st.UserID, st.AdventureID).Scan(&st.Sort)
		}
		res, err := s.DB.ExecContext(ctx, `INSERT INTO stages (user_id, adventure_id, sort, title, story, photo_id, lock, puzzle_id, answers, lock_hint, hints, hidden,
			place_name, lat, lng, place_ref, unlock, props, code) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			st.UserID, st.AdventureID, st.Sort, st.Title, st.Story, nullI(st.PhotoID), st.Lock, nullI(st.PuzzleID), toJSON(st.Answers), st.LockHint, toJSON(st.Hints), st.Hidden,
			st.PlaceName, nullF(st.Lat), nullF(st.Lng), st.PlaceRef, st.Unlock, st.Props, st.Code)
		if err != nil {
			return err
		}
		st.ID, _ = res.LastInsertId()
	} else {
		res, err := s.DB.ExecContext(ctx, `UPDATE stages SET sort = ?, title = ?, story = ?, photo_id = ?, lock = ?, puzzle_id = ?, answers = ?, lock_hint = ?, hints = ?, hidden = ?,
			place_name = ?, lat = ?, lng = ?, place_ref = ?, unlock = ?, props = ?, code = ? WHERE user_id = ? AND id = ?`,
			st.Sort, st.Title, st.Story, nullI(st.PhotoID), st.Lock, nullI(st.PuzzleID), toJSON(st.Answers), st.LockHint, toJSON(st.Hints), st.Hidden,
			st.PlaceName, nullF(st.Lat), nullF(st.Lng), st.PlaceRef, st.Unlock, st.Props, st.Code, st.UserID, st.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
	}
	_, _ = s.DB.ExecContext(ctx, `UPDATE adventures SET updated_at = ? WHERE user_id = ? AND id = ?`, Now(), st.UserID, st.AdventureID)
	return nil
}

// DeleteStage removes a stage.
func (s *Store) DeleteStage(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM stages WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ReorderStages puts an adventure's stages in this order.
func (s *Store) ReorderStages(ctx context.Context, uid, advID int64, ids []int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for i, id := range ids {
		if _, err := tx.ExecContext(ctx, `UPDATE stages SET sort = ? WHERE user_id = ? AND adventure_id = ? AND id = ?`, i+1, uid, advID, id); err != nil {
			return err
		}
	}
	return tx.Commit()
}
