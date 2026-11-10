package store

import (
	"context"
	"encoding/json"
)

// ---- puzzle books ----

func scanBook(sc scanner) (*Book, error) {
	var b Book
	var large, share int
	if err := sc.Scan(&b.ID, &b.UserID, &b.Title, &b.Subtitle, &b.Notes, &b.Answers, &large, &b.ShareToken, &share, &b.CreatedAt, &b.UpdatedAt); err != nil {
		return nil, err
	}
	b.Large, b.ShareOn = large == 1, share == 1
	b.Items = []BookItem{}
	return &b, nil
}

const bookCols = `id, user_id, title, subtitle, notes, answers, large, share_token, share_on, created_at, updated_at`

// ListBooks lists a person's books with their puzzles.
func (s *Store) ListBooks(ctx context.Context, uid int64) ([]Book, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+bookCols+` FROM books WHERE user_id = ? ORDER BY updated_at DESC`, uid)
	if err != nil {
		return nil, err
	}
	out := []Book{}
	for rows.Next() {
		b, err := scanBook(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, *b)
	}
	rows.Close()
	for i := range out {
		out[i].Items, _ = s.bookItems(ctx, uid, out[i].ID)
	}
	return out, nil
}

func (s *Store) bookItems(ctx context.Context, uid, bookID int64) ([]BookItem, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT i.id, i.puzzle_id, i.sort, p.title, p.kind, p.level, p.size_text FROM book_items i JOIN puzzles p ON p.id = i.puzzle_id
		WHERE i.user_id = ? AND i.book_id = ? ORDER BY i.sort, i.id`, uid, bookID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []BookItem{}
	for rows.Next() {
		var it BookItem
		if err := rows.Scan(&it.ID, &it.PuzzleID, &it.Sort, &it.Title, &it.Kind, &it.Level, &it.SizeText); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// GetBook returns a book with its puzzles in order.
func (s *Store) GetBook(ctx context.Context, uid, id int64) (*Book, error) {
	b, err := scanBook(s.DB.QueryRowContext(ctx, `SELECT `+bookCols+` FROM books WHERE user_id = ? AND id = ?`, uid, id))
	if err != nil {
		return nil, notFound(err)
	}
	b.Items, err = s.bookItems(ctx, uid, id)
	return b, err
}

// BookByShare finds a shared book by its link.
func (s *Store) BookByShare(ctx context.Context, token string) (*Book, error) {
	if len(token) < 16 {
		return nil, ErrNotFound
	}
	b, err := scanBook(s.DB.QueryRowContext(ctx, `SELECT `+bookCols+` FROM books WHERE share_token = ? AND share_on = 1`, token))
	if err != nil {
		return nil, notFound(err)
	}
	b.Items, err = s.bookItems(ctx, b.UserID, b.ID)
	return b, err
}

// SaveBook creates or updates a book and sets its puzzles (only the person's own).
func (s *Store) SaveBook(ctx context.Context, b *Book, puzzleIDs []int64) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if b.ID == 0 {
		res, err := tx.ExecContext(ctx, `INSERT INTO books (user_id, title, subtitle, notes, answers, large, share_token, share_on) VALUES (?,?,?,?,?,?,?,?)`,
			b.UserID, b.Title, b.Subtitle, b.Notes, b.Answers, boolInt(b.Large), b.ShareToken, boolInt(b.ShareOn))
		if err != nil {
			return conflict(err)
		}
		b.ID, _ = res.LastInsertId()
	} else {
		res, err := tx.ExecContext(ctx, `UPDATE books SET title = ?, subtitle = ?, notes = ?, answers = ?, large = ?, share_on = ?, updated_at = ? WHERE user_id = ? AND id = ?`,
			b.Title, b.Subtitle, b.Notes, b.Answers, boolInt(b.Large), boolInt(b.ShareOn), Now(), b.UserID, b.ID)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return ErrNotFound
		}
	}
	if puzzleIDs != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM book_items WHERE user_id = ? AND book_id = ?`, b.UserID, b.ID); err != nil {
			return err
		}
		for i, pid := range puzzleIDs {
			if _, err := tx.ExecContext(ctx, `INSERT INTO book_items (user_id, book_id, puzzle_id, sort) SELECT ?, ?, id, ? FROM puzzles WHERE user_id = ? AND id = ?`,
				b.UserID, b.ID, i, b.UserID, pid); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

// DeleteBook removes a book (its puzzles stay).
func (s *Store) DeleteBook(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM books WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ---- word lists ----

const wordlistCols = `id, user_id, name, language, words, source, created_at, updated_at`

func scanWordList(sc scanner) (*WordList, error) {
	var w WordList
	var words string
	if err := sc.Scan(&w.ID, &w.UserID, &w.Name, &w.Language, &words, &w.Source, &w.CreatedAt, &w.UpdatedAt); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(words), &w.Words)
	if w.Words == nil {
		w.Words = []WordClue{}
	}
	w.Count = len(w.Words)
	return &w, nil
}

// ListWordLists lists a person's word lists.
func (s *Store) ListWordLists(ctx context.Context, uid int64) ([]WordList, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+wordlistCols+` FROM wordlists WHERE user_id = ? ORDER BY name COLLATE NOCASE`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WordList{}
	for rows.Next() {
		w, err := scanWordList(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *w)
	}
	return out, rows.Err()
}

// GetWordList returns one word list.
func (s *Store) GetWordList(ctx context.Context, uid, id int64) (*WordList, error) {
	w, err := scanWordList(s.DB.QueryRowContext(ctx, `SELECT `+wordlistCols+` FROM wordlists WHERE user_id = ? AND id = ?`, uid, id))
	return w, notFound(err)
}

// SaveWordList creates or updates a word list.
func (s *Store) SaveWordList(ctx context.Context, w *WordList) error {
	if w.Words == nil {
		w.Words = []WordClue{}
	}
	if w.ID == 0 {
		res, err := s.DB.ExecContext(ctx, `INSERT INTO wordlists (user_id, name, language, words, source) VALUES (?,?,?,?,?)`, w.UserID, w.Name, w.Language, toJSON(w.Words), w.Source)
		if err != nil {
			return err
		}
		w.ID, _ = res.LastInsertId()
		return nil
	}
	res, err := s.DB.ExecContext(ctx, `UPDATE wordlists SET name = ?, language = ?, words = ?, updated_at = ? WHERE user_id = ? AND id = ?`, w.Name, w.Language, toJSON(w.Words), Now(), w.UserID, w.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteWordList removes a word list.
func (s *Store) DeleteWordList(ctx context.Context, uid, id int64) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM wordlists WHERE user_id = ? AND id = ?`, uid, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
