package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// ---- everything out and back in ----

type exportPhoto struct {
	ID   int64  `json:"id"`
	Kind string `json:"kind"`
	Mime string `json:"mime"`
	Data string `json:"data"` // base64
}

type exportDoc struct {
	App        string            `json:"app"`
	Version    string            `json:"version"`
	ExportedAt string            `json:"exported_at"`
	Puzzles    []store.Puzzle    `json:"puzzles"`
	Books      []store.Book      `json:"books"`
	Adventures []store.Adventure `json:"adventures"`
	WordLists  []store.WordList  `json:"wordlists"`
	Photos     []exportPhoto     `json:"photos,omitempty"`
}

func (s *Server) export(ctx context.Context, uid int64, photos bool) (exportDoc, error) {
	doc := exportDoc{App: "pocket-puzzlecraft", Version: Version, ExportedAt: store.Now()}
	list, err := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{})
	if err != nil {
		return doc, err
	}
	arch, _ := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Archived: true})
	for _, p := range append(list, arch...) {
		full, err := s.store.GetPuzzle(ctx, uid, p.ID)
		if err == nil {
			doc.Puzzles = append(doc.Puzzles, *full)
		}
	}
	doc.Books, _ = s.store.ListBooks(ctx, uid)
	advs, _ := s.store.ListAdventures(ctx, uid)
	for _, a := range advs {
		if full, err := s.store.GetAdventure(ctx, uid, a.ID); err == nil {
			doc.Adventures = append(doc.Adventures, *full)
		}
	}
	doc.WordLists, _ = s.store.ListWordLists(ctx, uid)
	if photos {
		ids, _ := s.store.ListPhotoIDs(ctx, uid)
		for _, id := range ids {
			if p, err := s.store.GetPhoto(ctx, uid, id, false); err == nil {
				doc.Photos = append(doc.Photos, exportPhoto{ID: p.ID, Kind: p.Kind, Mime: p.Mime, Data: base64.StdEncoding.EncodeToString(p.Data)})
			}
		}
	}
	return doc, nil
}

func (s *Server) handleExportJSON(w http.ResponseWriter, r *http.Request) {
	doc, err := s.export(r.Context(), actorID(r.Context()), r.URL.Query().Get("photos") != "0")
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-puzzlecraft-%s.json"`, time.Now().Format("20060102")))
	writeJSON(w, http.StatusOK, doc)
}

func (s *Server) handleExportCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	list, err := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{})
	if err != nil {
		s.fail(w, err, "export")
		return
	}
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="pocket-puzzlecraft-%s.csv"`, time.Now().Format("20060102")))
	cw := csv.NewWriter(w)
	_ = cw.Write([]string{"id", "kind", "title", "level", "language", "size", "tags", "ready", "shared", "plays", "solved", "best_time", "created", "updated", "link"})
	for _, p := range list {
		best := ""
		if p.BestMs > 0 {
			best = duration(p.BestMs)
		}
		share := ""
		if p.ShareOn {
			share = s.link("/p/" + p.ShareToken)
		}
		_ = cw.Write([]string{strconv.FormatInt(p.ID, 10), p.Kind, p.Title, p.Level, p.Language, p.SizeText, strings.Join(p.Tags, ", "), strconv.FormatBool(p.Ready),
			share, strconv.Itoa(p.Plays), strconv.Itoa(p.Solves), best, p.CreatedAt[:10], p.UpdatedAt[:10], s.link(fmt.Sprintf("/puzzles/%d", p.ID))})
	}
	cw.Flush()
}

func readUpload(w http.ResponseWriter, r *http.Request) ([]byte, string, error) {
	r.Body = http.MaxBytesReader(w, r.Body, 128<<20)
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
		f, h, err := r.FormFile("file")
		if err != nil {
			return nil, "", err
		}
		defer f.Close()
		b, err := io.ReadAll(f)
		return b, h.Filename, err
	}
	b, err := io.ReadAll(r.Body)
	return b, r.URL.Query().Get("name"), err
}

// handleImport brings back a Puzzlecraft export, or adds a crossword file
// (.ipuz, Across Lite .puz) or a sudoku (.ipuz, or 81 digits as text).
func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	data, name, err := readUpload(w, r)
	if err != nil || len(data) == 0 {
		writeErr(w, http.StatusBadRequest, "send a file")
		return
	}
	ext := strings.ToLower(filepath.Ext(name))
	var p *store.Puzzle
	switch {
	case ext == ".puz" || bytes.Contains(data[:min(len(data), 64)], []byte("ACROSS&DOWN")):
		p, err = fromPUZ(data)
	case ext == ".ipuz" || bytes.Contains(data[:min(len(data), 400)], []byte("ipuz.org")):
		p, err = fromIPUZ(data)
	case ext == ".json" || bytes.HasPrefix(bytes.TrimSpace(data), []byte("{")):
		var doc exportDoc
		if err := json.Unmarshal(data, &doc); err != nil || doc.App != "pocket-puzzlecraft" {
			writeErr(w, http.StatusBadRequest, "this is not a Pocket Puzzlecraft export or a crossword file")
			return
		}
		res, err := s.importPocket(ctx, uid, doc)
		if err != nil {
			s.fail(w, err, "import")
			return
		}
		writeJSON(w, http.StatusOK, res)
		return
	default:
		// a sudoku as 81 digits (0 or . for empty), the way sudoku sites share them
		digits := []int{}
		for _, ch := range string(data) {
			switch {
			case ch >= '1' && ch <= '9':
				digits = append(digits, int(ch-'0'))
			case ch == '0' || ch == '.':
				digits = append(digits, 0)
			}
		}
		if len(digits) != 81 {
			writeErr(w, http.StatusBadRequest, "send a Pocket Puzzlecraft export, a .ipuz or .puz crossword, or a sudoku as 81 digits")
			return
		}
		raw, _ := puzzle.Clean("sudoku", mustJSON(&puzzle.Sudoku{Size: 9, Givens: digits}))
		p = &store.Puzzle{Kind: "sudoku", Title: "Sudoku", Data: raw}
	}
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	st := s.settingsOf(ctx, uid)
	p.UserID, p.ShareToken, p.Board, p.Reveal, p.Timer, p.Level = uid, newToken(), st.ShareBoard, true, true, firstNonEmpty(p.Level, st.Level)
	p.Language = firstNonEmpty(p.Language, "en")
	p.Tags = []string{"imported"}
	if err := prepare(p); err != nil {
		s.userFail(w, err, "import")
		return
	}
	if err := s.store.CreatePuzzle(ctx, p); err != nil {
		s.fail(w, err, "import")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "puzzle_id": p.ID, "text": "Added “" + p.Title + "”", "puzzles": 1})
}

// importPocket adds everything from an export; running it twice adds nothing twice.
func (s *Server) importPocket(ctx context.Context, uid int64, doc exportDoc) (map[string]any, error) {
	photoMap := map[int64]int64{}
	for _, ph := range doc.Photos {
		b, err := base64.StdEncoding.DecodeString(ph.Data)
		if err != nil || len(b) == 0 {
			continue
		}
		key := fmt.Sprintf("import:photo:%d:%x", uid, sha8(b))
		if v, _, ok := s.store.CacheGet(ctx, key); ok {
			if id, _ := strconv.ParseInt(v, 10, 64); id > 0 && s.store.OwnsPhoto(ctx, uid, id) {
				photoMap[ph.ID] = id
				continue
			}
		}
		p, err := s.savePhoto(ctx, uid, ph.Kind, b)
		if err != nil {
			continue
		}
		photoMap[ph.ID] = p.ID
		_ = s.store.CachePut(ctx, key, strconv.FormatInt(p.ID, 10))
	}
	mapPhoto := func(id *int64) *int64 {
		if id == nil {
			return nil
		}
		if n, ok := photoMap[*id]; ok {
			return &n
		}
		return nil
	}
	existing, _ := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{})
	arch, _ := s.store.ListPuzzles(ctx, uid, store.PuzzleFilter{Archived: true})
	have := map[string]int64{}
	for _, p := range append(existing, arch...) {
		have[p.Kind+"\x00"+p.Title+"\x00"+p.CreatedAt] = p.ID
	}
	idMap := map[int64]int64{}
	added := 0
	for _, p := range doc.Puzzles {
		k := p.Kind + "\x00" + p.Title + "\x00" + p.CreatedAt
		if id, ok := have[k]; ok {
			idMap[p.ID] = id
			continue
		}
		if !oneOf(p.Kind, puzzle.Kinds) {
			continue
		}
		old := p.ID
		p.ID, p.UserID, p.ShareToken, p.ShareOn, p.Source = 0, uid, newToken(), false, ""
		p.PhotoID = mapPhoto(p.PhotoID)
		p.Data = remapPhotos(p.Kind, p.Data, photoMap)
		if err := prepare(&p); err != nil {
			continue
		}
		created := p.CreatedAt
		if err := s.store.CreatePuzzle(ctx, &p); err != nil {
			return nil, err
		}
		if created != "" {
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE puzzles SET created_at = ? WHERE user_id = ? AND id = ?`, created, uid, p.ID)
		}
		idMap[old] = p.ID
		added++
	}
	books := 0
	haveBooks, _ := s.store.ListBooks(ctx, uid)
	for _, b := range doc.Books {
		dup := false
		for _, hb := range haveBooks {
			if hb.Title == b.Title && hb.CreatedAt == b.CreatedAt {
				dup = true
			}
		}
		if dup {
			continue
		}
		ids := []int64{}
		for _, it := range b.Items {
			if n, ok := idMap[it.PuzzleID]; ok {
				ids = append(ids, n)
			}
		}
		nb := store.Book{UserID: uid, Title: b.Title, Subtitle: b.Subtitle, Notes: b.Notes, Answers: firstNonEmpty(b.Answers, "back"), Large: b.Large, ShareToken: newToken()}
		if err := s.store.SaveBook(ctx, &nb, ids); err == nil {
			books++
			if b.CreatedAt != "" {
				_, _ = s.store.DB.ExecContext(ctx, `UPDATE books SET created_at = ? WHERE user_id = ? AND id = ?`, b.CreatedAt, uid, nb.ID)
			}
		}
	}
	advs := 0
	haveAdv, _ := s.store.ListAdventures(ctx, uid)
	for _, a := range doc.Adventures {
		dup := false
		for _, ha := range haveAdv {
			if ha.Title == a.Title && ha.CreatedAt == a.CreatedAt {
				dup = true
			}
		}
		if dup {
			continue
		}
		stages, created := a.Stages, a.CreatedAt
		a.ID, a.UserID, a.ShareToken, a.ShareOn, a.CalendarRef, a.CalendarURL, a.Source = 0, uid, newToken(), false, "", "", ""
		a.PhotoID = mapPhoto(a.PhotoID)
		if err := s.store.SaveAdventure(ctx, &a); err != nil {
			continue
		}
		if created != "" {
			_, _ = s.store.DB.ExecContext(ctx, `UPDATE adventures SET created_at = ? WHERE user_id = ? AND id = ?`, created, uid, a.ID)
		}
		for _, st := range stages {
			st.ID, st.UserID, st.AdventureID = 0, uid, a.ID
			st.PhotoID = mapPhoto(st.PhotoID)
			if st.PuzzleID != nil {
				if n, ok := idMap[*st.PuzzleID]; ok {
					st.PuzzleID = &n
				} else {
					st.PuzzleID = nil
				}
			}
			_ = s.store.SaveStage(ctx, &st)
		}
		advs++
	}
	lists := 0
	haveLists, _ := s.store.ListWordLists(ctx, uid)
	for _, l := range doc.WordLists {
		dup := false
		for _, hl := range haveLists {
			if hl.Name == l.Name && hl.Count == len(l.Words) {
				dup = true
			}
		}
		if dup {
			continue
		}
		l.ID, l.UserID, l.Source = 0, uid, ""
		if err := s.store.SaveWordList(ctx, &l); err == nil {
			lists++
		}
	}
	text := fmt.Sprintf("Added %d %s, %d %s, %d %s and %d word %s", added, plural(added, "puzzle", "puzzles"), books, plural(books, "book", "books"),
		advs, plural(advs, "adventure", "adventures"), lists, plural(lists, "list", "lists"))
	return map[string]any{"ok": true, "puzzles": added, "books": books, "adventures": advs, "wordlists": lists, "text": text}, nil
}

func sha8(b []byte) uint64 {
	var h uint64 = 1469598103934665603
	for _, c := range b {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

// remapPhotos points a jigsaw's or quiz's pictures at their new ids.
func remapPhotos(kind string, data json.RawMessage, m map[int64]int64) json.RawMessage {
	switch kind {
	case "jigsaw":
		var j puzzle.Jigsaw
		if json.Unmarshal(data, &j) == nil {
			j.PhotoID = m[j.PhotoID]
			return mustJSON(j)
		}
	case "quiz":
		var q puzzle.Quiz
		if json.Unmarshal(data, &q) == nil {
			for i := range q.Questions {
				if q.Questions[i].PhotoID != 0 {
					q.Questions[i].PhotoID = m[q.Questions[i].PhotoID]
				}
			}
			return mustJSON(q)
		}
	}
	return data
}

// ---- .ipuz (the open puzzle format) ----

func (s *Server) handleExportIPUZ(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetPuzzle(r.Context(), actorID(r.Context()), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	b, err := toIPUZ(p)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeFileName(p.Title)+`.ipuz"`)
	_, _ = w.Write(b)
}

func (s *Server) handleExportPUZ(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetPuzzle(r.Context(), actorID(r.Context()), idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	b, err := toPUZ(p)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+safeFileName(p.Title)+`.puz"`)
	_, _ = w.Write(b)
}

func toIPUZ(p *store.Puzzle) ([]byte, error) {
	doc := map[string]any{"version": "http://ipuz.org/v2", "title": p.Title, "author": "", "notes": p.Instructions, "origin": "Pocket Puzzlecraft", "difficulty": p.Level}
	switch p.Kind {
	case "crossword":
		c, err := decodeCrossword(p.Data)
		if err != nil {
			return nil, err
		}
		doc["kind"] = []string{"http://ipuz.org/crossword#1"}
		doc["dimensions"] = map[string]int{"width": c.W, "height": c.H}
		nums := map[int]int{}
		es := puzzle.Entries(c)
		for _, e := range es {
			nums[e.Cells[0]] = e.Num
		}
		circ := map[int]bool{}
		for _, i := range c.Circles {
			circ[i] = true
		}
		grid, sol := [][]any{}, [][]any{}
		for r := 0; r < c.H; r++ {
			gr, sr := []any{}, []any{}
			for col := 0; col < c.W; col++ {
				i := r*c.W + col
				v := c.Cells[i]
				if v == "#" || (c.Style == "freeform" && v == "") {
					if c.Style == "freeform" {
						gr, sr = append(gr, nil), append(sr, nil)
					} else {
						gr, sr = append(gr, "#"), append(sr, "#")
					}
					continue
				}
				var cell any = nums[i]
				if circ[i] {
					cell = map[string]any{"cell": nums[i], "style": map[string]any{"shapebg": "circle"}}
				}
				gr, sr = append(gr, cell), append(sr, v)
			}
			grid, sol = append(grid, gr), append(sol, sr)
		}
		doc["puzzle"], doc["solution"] = grid, sol
		across, down := [][]any{}, [][]any{}
		for _, e := range es {
			if e.Dir == "A" {
				across = append(across, []any{e.Num, e.Clue})
			} else {
				down = append(down, []any{e.Num, e.Clue})
			}
		}
		doc["clues"] = map[string]any{"Across": across, "Down": down}
	case "sudoku":
		var sd puzzle.Sudoku
		if err := json.Unmarshal(p.Data, &sd); err != nil {
			return nil, err
		}
		doc["kind"] = []string{"http://ipuz.org/sudoku#1"}
		doc["dimensions"] = map[string]int{"width": sd.Size, "height": sd.Size}
		grid, sol := [][]int{}, [][]int{}
		for r := 0; r < sd.Size; r++ {
			grid = append(grid, sd.Givens[r*sd.Size:(r+1)*sd.Size])
			sol = append(sol, sd.Solution[r*sd.Size:(r+1)*sd.Size])
		}
		doc["puzzle"], doc["solution"] = grid, sol
	default:
		return nil, errors.New("only crosswords and sudoku go into .ipuz files")
	}
	return json.MarshalIndent(doc, "", "  ")
}

func fromIPUZ(data []byte) (*store.Puzzle, error) {
	data = bytes.TrimSpace(data)
	// some files wrap the JSON in ipuz(...)
	if bytes.HasPrefix(data, []byte("ipuz(")) {
		data = bytes.TrimSuffix(bytes.TrimPrefix(data, []byte("ipuz(")), []byte(")"))
	}
	var doc struct {
		Kind       []string                    `json:"kind"`
		Title      string                      `json:"title"`
		Notes      string                      `json:"notes"`
		Dimensions struct{ Width, Height int } `json:"dimensions"`
		Puzzle     [][]any                     `json:"puzzle"`
		Solution   [][]any                     `json:"solution"`
		Clues      map[string][]any            `json:"clues"`
		Block      string                      `json:"block"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, errors.New("this .ipuz file could not be read")
	}
	kind := strings.Join(doc.Kind, " ")
	w, h := doc.Dimensions.Width, doc.Dimensions.Height
	block := firstNonEmpty(doc.Block, "#")
	cellStr := func(v any) string {
		switch t := v.(type) {
		case string:
			return t
		case float64:
			return strconv.Itoa(int(t))
		case map[string]any:
			if x, ok := t["value"]; ok {
				return fmt.Sprint(x)
			}
			if x, ok := t["cell"]; ok {
				return fmt.Sprint(x)
			}
		}
		return ""
	}
	at := func(g [][]any, r, c int) any {
		if r < len(g) && c < len(g[r]) {
			return g[r][c]
		}
		return nil
	}
	switch {
	case strings.Contains(kind, "crossword"):
		if w < 3 || h < 3 || w > 30 || h > 30 {
			return nil, errors.New("the crossword is too small or too big")
		}
		c := &puzzle.Crossword{W: w, H: h, Cells: make([]string, w*h), Clues: map[string]string{}, Style: "classic", Symmetry: "none", Language: "en"}
		free := false
		for r := 0; r < h; r++ {
			for col := 0; col < w; col++ {
				pv := at(doc.Puzzle, r, col)
				sv := cellStr(at(doc.Solution, r, col))
				i := r*w + col
				if pv == nil {
					free = true
					c.Cells[i] = "#"
					continue
				}
				if cellStr(pv) == block || sv == block {
					c.Cells[i] = "#"
					continue
				}
				if m, ok := pv.(map[string]any); ok {
					if st, ok := m["style"].(map[string]any); ok && st["shapebg"] == "circle" {
						c.Circles = append(c.Circles, i)
					}
				}
				c.Cells[i] = strings.ToUpper(sv)
			}
		}
		if free {
			for i, v := range c.Cells {
				if v == "#" {
					c.Cells[i] = ""
				}
			}
			c.Style = "freeform"
		}
		for dirName, list := range doc.Clues {
			dir := "A"
			if strings.HasPrefix(strings.ToLower(dirName), "down") {
				dir = "D"
			}
			for _, x := range list {
				switch t := x.(type) {
				case []any:
					if len(t) >= 2 {
						c.Clues[cellStr(t[0])+dir] = fmt.Sprint(t[1])
					}
				case map[string]any:
					c.Clues[cellStr(t["number"])+dir] = fmt.Sprint(t["clue"])
				}
			}
		}
		return &store.Puzzle{Kind: "crossword", Title: firstNonEmpty(htmlUnescape(doc.Title), "Crossword"), Instructions: doc.Notes, Data: mustJSON(c)}, nil
	case strings.Contains(kind, "sudoku"):
		n := w
		if n != 4 && n != 6 && n != 9 {
			return nil, errors.New("only 4 × 4, 6 × 6 and 9 × 9 sudoku can be read")
		}
		sd := &puzzle.Sudoku{Size: n, Givens: make([]int, n*n), Solution: make([]int, n*n)}
		for r := 0; r < n; r++ {
			for col := 0; col < n; col++ {
				v, _ := strconv.Atoi(cellStr(at(doc.Puzzle, r, col)))
				sd.Givens[r*n+col] = v
				sv, _ := strconv.Atoi(cellStr(at(doc.Solution, r, col)))
				sd.Solution[r*n+col] = sv
			}
		}
		return &store.Puzzle{Kind: "sudoku", Title: firstNonEmpty(doc.Title, "Sudoku"), Data: mustJSON(sd)}, nil
	}
	return nil, errors.New("only crosswords and sudoku can be read from .ipuz files")
}

// ---- Across Lite .puz ----

func cksum(data []byte, c uint16) uint16 {
	for _, b := range data {
		if c&1 != 0 {
			c = c>>1 | 0x8000
		} else {
			c >>= 1
		}
		c += uint16(b)
	}
	return c
}

func latin1(s string) []byte {
	out := []byte{}
	for _, r := range s {
		if r < 256 {
			out = append(out, byte(r))
		} else {
			out = append(out, '?')
		}
	}
	return out
}

func toPUZ(p *store.Puzzle) ([]byte, error) {
	if p.Kind != "crossword" {
		return nil, errors.New("only crosswords go into .puz files")
	}
	c, err := decodeCrossword(p.Data)
	if err != nil {
		return nil, err
	}
	if c.W > 255 || c.H > 255 {
		return nil, errors.New("too big for .puz")
	}
	sol, grid := make([]byte, c.W*c.H), make([]byte, c.W*c.H)
	for i, v := range c.Cells {
		if v == "#" || v == "" && c.Style == "freeform" {
			sol[i], grid[i] = '.', '.'
			continue
		}
		l := latin1(puzzle.Fold(v))
		if len(l) == 0 || v == "" {
			return nil, errors.New("fill every square before saving a .puz file")
		}
		sol[i], grid[i] = l[0], '-'
	}
	es := puzzle.Entries(c)
	// clues in .puz order: by number, across before down
	sort.SliceStable(es, func(a, b int) bool {
		if es[a].Num != es[b].Num {
			return es[a].Num < es[b].Num
		}
		return es[a].Dir == "A"
	})
	title, author, copyright, notes := latin1(p.Title), latin1(""), latin1(""), latin1(p.Instructions)
	clues := [][]byte{}
	for _, e := range es {
		clues = append(clues, latin1(firstNonEmpty(e.Clue, "-")))
	}
	cib := make([]byte, 8)
	cib[0], cib[1] = byte(c.W), byte(c.H)
	binary.LittleEndian.PutUint16(cib[2:], uint16(len(clues)))
	binary.LittleEndian.PutUint16(cib[4:], 1)
	cCIB := cksum(cib, 0)
	text := func(c uint16) uint16 {
		if len(title) > 0 {
			c = cksum(append(append([]byte{}, title...), 0), c)
		}
		if len(author) > 0 {
			c = cksum(append(append([]byte{}, author...), 0), c)
		}
		if len(copyright) > 0 {
			c = cksum(append(append([]byte{}, copyright...), 0), c)
		}
		for _, cl := range clues {
			c = cksum(cl, c)
		}
		if len(notes) > 0 {
			c = cksum(append(append([]byte{}, notes...), 0), c)
		}
		return c
	}
	global := text(cksum(grid, cksum(sol, cCIB)))
	cSol, cGrid, cPart := cksum(sol, 0), cksum(grid, 0), text(0)
	var b bytes.Buffer
	b.Write([]byte{byte(global), byte(global >> 8)})
	b.WriteString("ACROSS&DOWN\x00")
	b.Write([]byte{byte(cCIB), byte(cCIB >> 8)})
	mask := "ICHEATED"
	b.Write([]byte{mask[0] ^ byte(cCIB), mask[1] ^ byte(cSol), mask[2] ^ byte(cGrid), mask[3] ^ byte(cPart)})
	b.Write([]byte{mask[4] ^ byte(cCIB>>8), mask[5] ^ byte(cSol>>8), mask[6] ^ byte(cGrid>>8), mask[7] ^ byte(cPart>>8)})
	b.WriteString("1.3\x00")
	b.Write(make([]byte, 2))  // reserved
	b.Write(make([]byte, 2))  // scrambled checksum
	b.Write(make([]byte, 12)) // reserved
	b.Write(cib)
	b.Write(sol)
	b.Write(grid)
	for _, s := range [][]byte{title, author, copyright} {
		b.Write(s)
		b.WriteByte(0)
	}
	for _, cl := range clues {
		b.Write(cl)
		b.WriteByte(0)
	}
	b.Write(notes)
	b.WriteByte(0)
	return b.Bytes(), nil
}

func fromPUZ(data []byte) (*store.Puzzle, error) {
	i := bytes.Index(data, []byte("ACROSS&DOWN\x00"))
	if i < 2 {
		return nil, errors.New("this .puz file could not be read")
	}
	base := i - 2
	if len(data) < base+0x34 {
		return nil, errors.New("this .puz file is cut short")
	}
	w, h := int(data[base+0x2C]), int(data[base+0x2D])
	n := int(binary.LittleEndian.Uint16(data[base+0x2E:]))
	if binary.LittleEndian.Uint16(data[base+0x32:]) != 0 {
		return nil, errors.New("this crossword is locked (scrambled) — unlock it in Across Lite first")
	}
	if w < 3 || h < 3 || w > 30 || h > 30 {
		return nil, errors.New("the crossword is too small or too big")
	}
	off := base + 0x34
	if len(data) < off+2*w*h {
		return nil, errors.New("this .puz file is cut short")
	}
	sol := data[off : off+w*h]
	off += 2 * w * h
	strs := []string{}
	for off < len(data) && len(strs) < 3+n+1 {
		j := bytes.IndexByte(data[off:], 0)
		if j < 0 {
			break
		}
		raw := data[off : off+j]
		r := []rune{}
		for _, c := range raw {
			r = append(r, rune(c)) // ISO-8859-1
		}
		strs = append(strs, string(r))
		off += j + 1
	}
	if len(strs) < 3+n {
		return nil, errors.New("this .puz file's clues could not be read")
	}
	c := &puzzle.Crossword{W: w, H: h, Cells: make([]string, w*h), Clues: map[string]string{}, Style: "classic", Symmetry: "none", Language: "en"}
	for k, ch := range sol {
		if ch == '.' || ch == ':' {
			c.Cells[k] = "#"
		} else {
			c.Cells[k] = strings.ToUpper(string(rune(ch)))
		}
	}
	es := puzzle.Entries(c)
	sort.SliceStable(es, func(a, b int) bool {
		if es[a].Num != es[b].Num {
			return es[a].Num < es[b].Num
		}
		return es[a].Dir == "A"
	})
	for k, e := range es {
		if 3+k < len(strs) {
			c.Clues[e.Key] = strs[3+k]
		}
	}
	notes := ""
	if len(strs) > 3+n {
		notes = strs[3+n]
	}
	return &store.Puzzle{Kind: "crossword", Title: firstNonEmpty(strs[0], "Crossword"), Instructions: notes, Data: mustJSON(c)}, nil
}
