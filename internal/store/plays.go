package store

import (
	"context"
	"database/sql"
	"encoding/json"
)

const playCols = `id, user_id, what, ref_id, kind, day, via, player, play_key, started_at, finished_at, ms, hints, mistakes, score, max_score, stage, stage_at, state, last_at`

func scanPlay(sc scanner) (*Play, error) {
	var p Play
	var fin, stageAt sql.NullString
	var state string
	if err := sc.Scan(&p.ID, &p.UserID, &p.What, &p.RefID, &p.Kind, &p.Day, &p.Via, &p.Player, &p.Key, &p.StartedAt, &fin, &p.Ms, &p.Hints, &p.Mistakes,
		&p.Score, &p.MaxScore, &p.Stage, &stageAt, &state, &p.LastAt); err != nil {
		return nil, err
	}
	if fin.Valid {
		p.FinishedAt = &fin.String
	}
	if stageAt.Valid {
		p.StageAt = &stageAt.String
	}
	p.State = json.RawMessage(state)
	if !json.Valid(p.State) {
		p.State = json.RawMessage(`{}`)
	}
	return &p, nil
}

// CreatePlay starts a go.
func (s *Store) CreatePlay(ctx context.Context, p *Play) error {
	if len(p.State) == 0 {
		p.State = json.RawMessage(`{}`)
	}
	if p.StartedAt == "" {
		p.StartedAt = Now()
	}
	res, err := s.DB.ExecContext(ctx, `INSERT INTO plays (user_id, what, ref_id, kind, day, via, player, play_key, started_at, stage_at, state) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		p.UserID, p.What, p.RefID, p.Kind, p.Day, p.Via, p.Player, p.Key, p.StartedAt, p.StartedAt, string(p.State))
	if err != nil {
		return conflict(err)
	}
	p.ID, _ = res.LastInsertId()
	p.StageAt = &p.StartedAt
	return nil
}

// PlayByKey finds a go by the key the player's browser keeps.
func (s *Store) PlayByKey(ctx context.Context, key string) (*Play, error) {
	if len(key) < 16 {
		return nil, ErrNotFound
	}
	p, err := scanPlay(s.DB.QueryRowContext(ctx, `SELECT `+playCols+` FROM plays WHERE play_key = ?`, key))
	return p, notFound(err)
}

// UpdatePlay saves progress.
func (s *Store) UpdatePlay(ctx context.Context, p *Play) error {
	_, err := s.DB.ExecContext(ctx, `UPDATE plays SET player = ?, finished_at = ?, ms = ?, hints = ?, mistakes = ?, score = ?, max_score = ?, stage = ?, stage_at = ?, state = ?, last_at = ?
		WHERE user_id = ? AND id = ?`, p.Player, p.FinishedAt, p.Ms, p.Hints, p.Mistakes, p.Score, p.MaxScore, p.Stage, p.StageAt, string(p.State), Now(), p.UserID, p.ID)
	return err
}

// PlayFilter narrows plays.
type PlayFilter struct {
	What     string
	RefID    int64
	Via      string
	Finished *bool
	Day      string
	From, To string // dates of finishing (or starting)
	Limit    int
	Best     bool // fastest first
}

// ListPlays lists plays of a person's puzzles and adventures.
func (s *Store) ListPlays(ctx context.Context, uid int64, f PlayFilter) ([]Play, error) {
	q := `SELECT ` + playCols + ` FROM plays WHERE user_id = ?`
	args := []any{uid}
	if f.What != "" {
		q += ` AND what = ?`
		args = append(args, f.What)
	}
	if f.RefID > 0 {
		q += ` AND ref_id = ?`
		args = append(args, f.RefID)
	}
	if f.Via != "" {
		q += ` AND via = ?`
		args = append(args, f.Via)
	}
	if f.Day != "" {
		q += ` AND day = ?`
		args = append(args, f.Day)
	}
	if f.Finished != nil {
		if *f.Finished {
			q += ` AND finished_at IS NOT NULL`
		} else {
			q += ` AND finished_at IS NULL`
		}
	}
	if f.From != "" {
		q += ` AND substr(COALESCE(finished_at, started_at), 1, 10) >= ?`
		args = append(args, f.From)
	}
	if f.To != "" {
		q += ` AND substr(COALESCE(finished_at, started_at), 1, 10) <= ?`
		args = append(args, f.To)
	}
	if f.Best {
		q += ` ORDER BY finished_at IS NULL, stage DESC, ms + hints * 0 ASC, finished_at`
	} else {
		q += ` ORDER BY last_at DESC`
	}
	if f.Limit > 0 {
		q += ` LIMIT ?`
		args = append(args, f.Limit)
	}
	rows, err := s.DB.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Play{}
	for rows.Next() {
		p, err := scanPlay(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

// DeletePlay removes a go (a result someone does not want on the board).
func (s *Store) DeletePlay(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM plays WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// MyPlay is the person's own latest go at something.
func (s *Store) MyPlay(ctx context.Context, uid int64, what string, ref int64, day string) (*Play, error) {
	p, err := scanPlay(s.DB.QueryRowContext(ctx, `SELECT `+playCols+` FROM plays WHERE user_id = ? AND what = ? AND ref_id = ? AND day = ? AND via = 'me' ORDER BY id DESC LIMIT 1`,
		uid, what, ref, day))
	return p, notFound(err)
}

// SolvedDays lists the days the person solved at least one daily puzzle (for streaks).
func (s *Store) SolvedDays(ctx context.Context, uid int64) ([]string, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT DISTINCT substr(day, 1, 10) FROM plays WHERE user_id = ? AND what = 'daily' AND finished_at IS NOT NULL AND via = 'me' ORDER BY 1 DESC LIMIT 800`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var d string
		if rows.Scan(&d) == nil {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}
