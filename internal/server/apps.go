package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/puzzle"
	"github.com/abjelosevic88/pocket-homelab-puzzlecraft/internal/store"
)

// The other Pocket apps. None of it is needed: when an app is not there,
// the page greys the button and says what to do.

func linkTo(sib *siblingConn, path string) string {
	if sib == nil || sib.public == "" {
		return ""
	}
	return strings.TrimRight(sib.public, "/") + path
}

func plainErr(a pocketApp, err error) string {
	if isNotConnected(err) {
		return err.Error()
	}
	return a.name + ": " + err.Error()
}

func (s *Server) appFail(w http.ResponseWriter, a pocketApp, err error) {
	if isNotConnected(err) {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeErr(w, http.StatusBadGateway, plainErr(a, err))
}

// handleApps lists how the person reaches each app, in words they can act on.
func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	c := s.clients(ctx, actorID(ctx))
	out := []appState{}
	for _, a := range pocketApps {
		out = append(out, c.status(ctx, a))
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": out, "core": s.core.Configured()})
}

func (s *Server) sib(w http.ResponseWriter, r *http.Request, a pocketApp, scopes ...string) (*siblingConn, bool) {
	ctx := r.Context()
	sib, err := s.clients(ctx, actorID(ctx)).conn(ctx, a, scopes...)
	if err != nil {
		s.appFail(w, a, err)
		return nil, false
	}
	return sib, true
}

// shareable is something that can be sent: its title, a line about it, its link.
type shareable struct {
	what, title, sub, url, icon string
	shared                      bool
	date, time                  string
}

// shareableOf finds what is being sent ("puzzle", "adventure" or "book").
func (s *Server) shareableOf(ctx context.Context, uid int64, what string, id int64) (*shareable, error) {
	switch what {
	case "puzzle":
		p, err := s.store.GetPuzzle(ctx, uid, id)
		if err != nil {
			return nil, err
		}
		return &shareable{what: what, title: p.Title, sub: puzzle.KindName[p.Kind] + " · " + p.SizeText, url: s.link("/p/" + p.ShareToken), shared: p.ShareOn, icon: kindEmoji[p.Kind]}, nil
	case "adventure":
		a, err := s.store.GetAdventure(ctx, uid, id)
		if err != nil {
			return nil, err
		}
		sub := map[string]string{"escape": "Escape room", "hunt": "Treasure hunt", "mystery": "Mystery", "party": "Party game"}[a.Kind]
		if len(a.Stages) > 0 {
			sub += fmt.Sprintf(" · %d stages", len(a.Stages))
		}
		if a.Minutes > 0 {
			sub += fmt.Sprintf(" · %d minutes", a.Minutes)
		}
		return &shareable{what: what, title: a.Title, sub: sub, url: s.link("/a/" + a.ShareToken), shared: a.ShareOn, icon: "🗝️", date: a.Date, time: a.Time}, nil
	case "book":
		b, err := s.store.GetBook(ctx, uid, id)
		if err != nil {
			return nil, err
		}
		return &shareable{what: what, title: b.Title, sub: fmt.Sprintf("Puzzle book · %d puzzles", len(b.Items)), url: s.link("/b/" + b.ShareToken), shared: b.ShareOn, icon: "📚"}, nil
	}
	return nil, store.ErrNotFound
}

var kindEmoji = map[string]string{"crossword": "✏️", "wordsearch": "🔎", "sudoku": "🔢", "nonogram": "🖼️", "cryptogram": "🔐", "scramble": "🔀", "quiz": "❓",
	"maze": "🌀", "logic": "🧠", "ladder": "🪜", "jigsaw": "🧩"}

type sendIn struct {
	What string `json:"what"`
	ID   int64  `json:"id"`
	Text string `json:"text"`
	To   string `json:"to"`
}

func (s *Server) sendTarget(w http.ResponseWriter, r *http.Request) (*sendIn, *shareable, bool) {
	var in sendIn
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return nil, nil, false
	}
	sh, err := s.shareableOf(r.Context(), actorID(r.Context()), in.What, in.ID)
	if err != nil {
		s.fail(w, err, in.What)
		return nil, nil, false
	}
	if !sh.shared {
		writeErr(w, http.StatusBadRequest, "switch its share link on first, so the people you send it to can play")
		return nil, nil, false
	}
	return &in, sh, true
}

// ---- Pocket Chat ----

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appChat, scopeChatRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/conversations", nil)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "conversations") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "name"), str(m, "title")), "kind": str(m, "kind"), "emoji": str(m, "emoji")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"conversations": out})
}

// handleShareChat posts a puzzle to a chat as a card with its play link.
func (s *Server) handleShareChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	in, sh, ok := s.sendTarget(w, r)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appChat, scopeChatWrite)
	if !ok {
		return
	}
	st := s.settingsOf(ctx, uid)
	card := map[string]any{"title": sh.title, "subtitle": sh.sub, "lines": []string{"Tap to play — no account needed."}, "icon": sh.icon, "url": sh.url, "app": "puzzlecraft", "kind": "card"}
	text := firstNonEmpty(strings.TrimSpace(in.Text), "Can you solve this one?")
	body := map[string]any{"to": firstNonEmpty(strings.TrimSpace(in.To), st.ChatTo, "me"), "text": clip(text, 2000), "card": card,
		"client_id": fmt.Sprintf("puzzlecraft-%d-%s-%d-%d", uid, in.What, in.ID, time.Now().Unix()/30)}
	res, err := postJSONTo(ctx, sib, "/api/v1/send", body)
	if err != nil {
		s.appFail(w, appChat, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": linkTo(sib, "/c/"+str(res, "conversation_id")), "text": "Sent to Pocket Chat"})
}

// ---- Pocket Email ----

func (s *Server) handleShareEmail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		What    string `json:"what"`
		ID      int64  `json:"id"`
		Text    string `json:"text"`
		Subject string `json:"subject"`
		To      []struct {
			Name string `json:"name"`
			Addr string `json:"addr"`
		} `json:"to"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	to := []map[string]string{}
	for _, t := range in.To {
		if a := strings.TrimSpace(t.Addr); strings.Contains(a, "@") && len(to) < 50 {
			to = append(to, map[string]string{"name": strings.TrimSpace(t.Name), "addr": a})
		}
	}
	if len(to) == 0 {
		writeErr(w, http.StatusBadRequest, "who should get it? Add at least one email address")
		return
	}
	sh, err := s.shareableOf(ctx, uid, in.What, in.ID)
	if err != nil {
		s.fail(w, err, in.What)
		return
	}
	if !sh.shared {
		writeErr(w, http.StatusBadRequest, "switch its share link on first, so the people you send it to can play")
		return
	}
	sib, ok := s.sib(w, r, appEmail, scopeMailSend)
	if !ok {
		return
	}
	note := strings.TrimSpace(in.Text)
	if note == "" {
		note = "I made a puzzle for you — can you solve it?"
		if in.What == "adventure" {
			note = "You are invited to play!"
		}
	}
	when := ""
	if sh.date != "" {
		when = "\n\nWhen: " + niceDate(sh.date)
		if sh.time != "" {
			when += " at " + sh.time
		}
	}
	textBody := fmt.Sprintf("%s\n\n%s — %s%s\n\nPlay it here (no account needed):\n%s\n", note, sh.title, sh.sub, when, sh.url)
	html := fmt.Sprintf(`<p>%s</p><p><b>%s</b><br><span style="color:#667">%s</span>%s</p><p><a href="%s" style="display:inline-block;padding:10px 18px;background:#1f7a8c;color:#fff;border-radius:8px;text-decoration:none">Play it</a></p><p style="color:#889;font-size:12px">No account needed. Made with Pocket Puzzlecraft.</p>`,
		htmlEsc(note), htmlEsc(sh.title), htmlEsc(sh.sub), strings.ReplaceAll(htmlEsc(when), "\n", "<br>"), sh.url)
	subject := firstNonEmpty(strings.TrimSpace(in.Subject), sh.title)
	res, err := postJSONTo(ctx, sib, "/api/v1/send", map[string]any{"to": to, "subject": clip(subject, 200), "body_text": textBody, "body_html": html})
	if err != nil {
		s.appFail(w, appEmail, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": fmt.Sprintf("Sent to %d %s", len(to), plural(len(to), "person", "people")), "status": str(res, "status")})
}

// ---- Pocket Contacts ----

func (s *Server) handleContacts(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appContacts, scopeContactsRead)
	if !ok {
		return
	}
	q := url.Values{"limit": {"40"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/contacts", q)
	if err != nil {
		s.appFail(w, appContacts, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "contacts") {
		emails := []string{}
		if arr, ok := m["emails"].([]any); ok {
			for _, e := range arr {
				if es, ok := e.(string); ok {
					emails = append(emails, es)
				}
			}
		}
		out = append(out, map[string]any{"id": idOf(m), "name": firstNonEmpty(str(m, "fn"), str(m, "given")), "given": str(m, "given"), "emails": emails,
			"birthday": str(m, "birthday"), "photo_url": "", "url": linkTo(sib, "/contacts/"+idOf(m))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"contacts": out})
}

// ---- Pocket Calendar ----

// handleToCalendar puts an adventure (or a puzzle night) in the calendar.
func (s *Server) handleToCalendar(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		What    string `json:"what"`
		ID      int64  `json:"id"`
		Date    string `json:"date"`
		Time    string `json:"time"`
		Minutes int    `json:"minutes"`
		Title   string `json:"title"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	sh, err := s.shareableOf(ctx, uid, in.What, in.ID)
	if err != nil {
		s.fail(w, err, in.What)
		return
	}
	date := firstNonEmpty(in.Date, sh.date)
	if date == "" || !validDate(date) {
		writeErr(w, http.StatusBadRequest, "pick a day first")
		return
	}
	tm := firstNonEmpty(in.Time, sh.time)
	mins := in.Minutes
	var adv *store.Adventure
	if in.What == "adventure" {
		adv, _ = s.store.GetAdventure(ctx, uid, in.ID)
		if mins == 0 && adv != nil {
			mins = adv.Minutes + 30
		}
	}
	if mins <= 0 {
		mins = 90
	}
	body := map[string]any{"title": clip(firstNonEmpty(in.Title, sh.title), 200), "description": sh.sub + "\n\nPlay: " + sh.url + "\nIn Puzzlecraft: " + s.link("/"+in.What+"s/"+strconv.FormatInt(in.ID, 10)),
		"url": sh.url, "categories": []string{"Puzzles"}}
	if adv != nil && adv.Place != "" {
		body["location"] = adv.Place
	}
	if tm != "" && reTime.MatchString(tm) {
		start, _ := time.ParseInLocation("2006-01-02 15:04", date+" "+tm, time.Local)
		body["start"], body["end"] = start.Format("2006-01-02T15:04"), start.Add(time.Duration(mins)*time.Minute).Format("2006-01-02T15:04")
	} else {
		body["all_day"], body["start"], body["end"] = true, date, date
	}
	sib, ok := s.sib(w, r, appCalendar, scopeCalendarWrite)
	if !ok {
		return
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/events", body)
	if err != nil {
		s.appFail(w, appCalendar, err)
		return
	}
	evURL := linkTo(sib, "/?event="+url.QueryEscape(firstNonEmpty(str(res, "event_id"), idOf(res))))
	if adv != nil {
		adv.CalendarRef, adv.CalendarURL = firstNonEmpty(str(res, "event_id"), idOf(res)), evURL
		if adv.Date == "" {
			adv.Date, adv.Time = date, tm
		}
		_ = s.store.SaveAdventure(ctx, adv)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": evURL, "text": "Added to Pocket Calendar"})
}

// ---- Pocket Photos ----

func (s *Server) handlePhotosList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	body := map[string]any{"filter": map[string]any{"trashedAt": map[string]any{"eq": nil}, "type": map[string]any{"eq": "IMAGE"}, "visibility": map[string]any{"eq": "timeline"}},
		"size": 60, "withExif": false, "orderBy": map[string]any{"field": "fileCreatedAt", "direction": "desc"}}
	path := "/api/v1/immich/search/metadata"
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		body["query"] = q
		path = "/api/v1/immich/search/smart"
	}
	if c := r.URL.Query().Get("cursor"); c != "" {
		body["cursor"] = c
	}
	res, err := postJSONTo(r.Context(), sib, path, body)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	assets := obj(res, "assets")
	out := []map[string]any{}
	for _, m := range rows(assets["items"]) {
		id := str(m, "id")
		if !reUUID.MatchString(id) {
			continue
		}
		out = append(out, map[string]any{"id": id, "name": str(m, "originalFileName"), "date": day10(m["fileCreatedAt"]), "thumb_url": "/api/v1/apps/photos/" + id + "/image"})
	}
	writeJSON(w, http.StatusOK, map[string]any{"photos": out, "cursor": assets["nextCursor"]})
}

func (s *Server) handlePhotoImage(w http.ResponseWriter, r *http.Request) {
	id := chiParam(r, "uuid")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	size := r.URL.Query().Get("size")
	if size != "preview" {
		size = "thumbnail"
	}
	b, ctype, err := sib.client.Bytes(r.Context(), "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {size}}, 16<<20)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	w.Header().Set("Content-Type", firstNonEmpty(ctype, http.DetectContentType(b)))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	_, _ = w.Write(b)
}

// handlePhotoCopy keeps a copy of a Pocket Photos picture here (for a jigsaw, a question, a stage).
func (s *Server) handlePhotoCopy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	id := chiParam(r, "uuid")
	if !reUUID.MatchString(id) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	sib, ok := s.sib(w, r, appPhotos, scopePhotosRead)
	if !ok {
		return
	}
	b, _, err := sib.client.Bytes(ctx, "/api/v1/immich/assets/"+id+"/thumbnail", url.Values{"size": {"preview"}}, maxPhoto)
	if err != nil {
		s.appFail(w, appPhotos, err)
		return
	}
	p, err := s.savePhoto(ctx, uid, firstNonEmpty(r.URL.Query().Get("kind"), "puzzle"), b)
	if err != nil {
		s.userFail(w, err, "photo")
		return
	}
	u, t := photoURL(&p.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"id": p.ID, "url": u, "thumb_url": t, "width": p.Width, "height": p.Height})
}

// ---- Pocket Files ----

// handleToFiles saves a printout (PDF/PNG/SVG, made in the browser) or a
// crossword file in Pocket Files.
func (s *Server) handleToFiles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	kind := r.URL.Query().Get("type")
	ctype := map[string]string{"svg": "image/svg+xml", "png": "image/png", "pdf": "application/pdf", "ipuz": "application/json", "puz": "application/octet-stream", "json": "application/json"}[kind]
	if ctype == "" {
		writeErr(w, http.StatusBadRequest, "send a PDF, PNG, SVG, .ipuz or .puz file")
		return
	}
	name := safeFileName(firstNonEmpty(r.URL.Query().Get("name"), "Puzzle"))
	var data []byte
	var err error
	if (kind == "ipuz" || kind == "puz") && queryInt(r, "puzzle") > 0 {
		p, gerr := s.store.GetPuzzle(ctx, uid, int64(queryInt(r, "puzzle")))
		if gerr != nil {
			s.fail(w, gerr, "puzzle")
			return
		}
		name = safeFileName(p.Title)
		if kind == "ipuz" {
			data, err = toIPUZ(p)
		} else {
			data, err = toPUZ(p)
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	} else {
		data, err = s.readBody(r)
		if err != nil || len(data) == 0 {
			writeErr(w, http.StatusBadRequest, "nothing to save")
			return
		}
	}
	sib, ok := s.sib(w, r, appFiles, scopeFilesWrite)
	if !ok {
		return
	}
	folder := strings.Trim(firstNonEmpty(s.settingsOf(ctx, uid).FilesFolder, "Puzzles"), "/")
	full := name + "." + kind
	var res map[string]any
	q := url.Values{"space": {"home"}, "path": {"/"}, "name": {folder + "/" + full}, "conflict": {"rename"}, "mtime": {strconv.FormatInt(time.Now().UnixMilli(), 10)}}
	if err := sib.client.PutRaw(ctx, "/api/v1/fs/upload", q, data, ctype, &res); err != nil {
		s.appFail(w, appFiles, err)
		return
	}
	path := firstNonEmpty(str(res, "path"), "/"+folder+"/"+full)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Saved in Pocket Files", "url": linkTo(sib, "/f/home"+path)})
}

func safeFileName(s string) string {
	s = strings.Map(func(r rune) rune {
		if strings.ContainsRune(`/\:*?"<>|`, r) || r < 32 {
			return '-'
		}
		return r
	}, s)
	return clip(strings.TrimSpace(s), 100)
}

// ---- Pocket Notes ----

func (s *Server) toNotes(ctx context.Context, uid int64, title, content string) (string, error) {
	sib, err := s.clients(ctx, uid).conn(ctx, appNotes, scopeNotesWrite)
	if err != nil {
		return "", err
	}
	folder := firstNonEmpty(s.settingsOf(ctx, uid).NotesFolder, "Puzzles")
	res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"title": clip(title, 200), "content": content, "category": folder})
	if err != nil {
		return "", fmt.Errorf("Pocket Notes: %w", err)
	}
	return linkTo(sib, "/notes/"+idOf(res)), nil
}

// handleToNotes saves a puzzle's answer key or an adventure's host sheet as a note.
func (s *Server) handleToNotes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		What string `json:"what"`
		ID   int64  `json:"id"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	var title, text string
	switch in.What {
	case "puzzle":
		p, err := s.store.GetPuzzle(ctx, uid, in.ID)
		if err != nil {
			s.fail(w, err, "puzzle")
			return
		}
		title, text = p.Title+" — answers", answerKey(p)+"\n\n"+s.link(fmt.Sprintf("/puzzles/%d", p.ID))
	case "adventure":
		a, err := s.store.GetAdventure(ctx, uid, in.ID)
		if err != nil {
			s.fail(w, err, "adventure")
			return
		}
		title, text = a.Title+" — host sheet", s.hostSheet(a)
	default:
		writeErr(w, http.StatusBadRequest, "send a puzzle or an adventure")
		return
	}
	u, err := s.toNotes(ctx, uid, title, text)
	if err != nil {
		s.userFail(w, err, "notes")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "url": u, "text": "Saved in Pocket Notes"})
}

// hostSheet is everything the host needs: each stage, its answer, where it is hidden.
func (s *Server) hostSheet(a *store.Adventure) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", a.Title)
	if a.Date != "" {
		fmt.Fprintf(&b, "When: %s %s\n", niceDate(a.Date), a.Time)
	}
	if a.Place != "" {
		fmt.Fprintf(&b, "Where: %s\n", a.Place)
	}
	if a.Minutes > 0 {
		fmt.Fprintf(&b, "Time limit: %d minutes\n", a.Minutes)
	}
	fmt.Fprintf(&b, "Players' link: %s\n\n", s.link("/a/"+a.ShareToken))
	if len(a.Prep) > 0 {
		b.WriteString("## Setting up\n\n")
		for _, p := range a.Prep {
			mark := "[ ]"
			if p.Done {
				mark = "[x]"
			}
			fmt.Fprintf(&b, "- %s %s\n", mark, p.Text)
		}
		b.WriteString("\n")
	}
	b.WriteString("## Stages\n\n")
	for i, st := range a.Stages {
		fmt.Fprintf(&b, "### %d. %s\n\n", i+1, firstNonEmpty(st.Title, "Stage"))
		lock := map[string]string{"answer": "Answer", "code": "Number lock", "word": "Word lock", "directions": "Direction lock", "colors": "Colour lock", "qr": "Find the QR card", "puzzle": "Puzzle", "none": "Nothing to open"}[st.Lock]
		fmt.Fprintf(&b, "- Lock: %s\n", lock)
		if len(st.Answers) > 0 {
			fmt.Fprintf(&b, "- Answer: **%s**\n", strings.Join(st.Answers, "** or **"))
		}
		if st.PuzzleTitle != "" {
			fmt.Fprintf(&b, "- Puzzle: %s\n", st.PuzzleTitle)
		}
		if st.Hidden != "" {
			fmt.Fprintf(&b, "- Hidden: %s\n", st.Hidden)
		}
		if st.PlaceName != "" {
			fmt.Fprintf(&b, "- Place: %s\n", st.PlaceName)
		}
		if st.Props != "" {
			fmt.Fprintf(&b, "- Props: %s\n", st.Props)
		}
		for k, h := range st.Hints {
			fmt.Fprintf(&b, "- Hint %d (after %d min): %s\n", k+1, h.After, h.Text)
		}
		b.WriteString("\n")
	}
	if a.Notes != "" {
		fmt.Fprintf(&b, "## Notes\n\n%s\n", a.Notes)
	}
	return b.String()
}

// answerKey writes a puzzle's answer in plain text.
func answerKey(p *store.Puzzle) string {
	var b strings.Builder
	switch p.Kind {
	case "crossword":
		c, _ := decodeCrossword(p.Data)
		for _, dir := range []string{"A", "D"} {
			b.WriteString(map[string]string{"A": "Across\n", "D": "\nDown\n"}[dir])
			for _, e := range puzzle.Entries(c) {
				if e.Dir == dir {
					fmt.Fprintf(&b, "%d. %s — %s\n", e.Num, e.Answer, e.Clue)
				}
			}
		}
	case "sudoku":
		var s puzzle.Sudoku
		_ = json.Unmarshal(p.Data, &s)
		for r := 0; r < s.Size; r++ {
			row := []string{}
			for c := 0; c < s.Size; c++ {
				row = append(row, strconv.Itoa(s.Solution[r*s.Size+c]))
			}
			b.WriteString(strings.Join(row, " ") + "\n")
		}
	case "wordsearch":
		var ws puzzle.WordSearch
		_ = json.Unmarshal(p.Data, &ws)
		for _, x := range ws.Words {
			if !x.Off {
				fmt.Fprintf(&b, "%s — row %d, column %d, going %s\n", x.Word, x.R+1, x.C+1, dirWord(x.DR, x.DC))
			}
		}
		if ws.Message != "" {
			fmt.Fprintf(&b, "\nHidden message: %s\n", ws.Message)
		}
	case "cryptogram":
		var c puzzle.Cryptogram
		_ = json.Unmarshal(p.Data, &c)
		b.WriteString(c.Text)
		if c.Author != "" {
			b.WriteString(" — " + c.Author)
		}
		b.WriteString("\n")
	case "scramble":
		var s puzzle.Scramble
		_ = json.Unmarshal(p.Data, &s)
		for i, x := range s.Words {
			fmt.Fprintf(&b, "%d. %s → %s\n", i+1, x.Scrambled, x.Word)
		}
		if s.Final != nil {
			fmt.Fprintf(&b, "\nFinal answer: %s\n", s.Final.Answer)
		}
	case "quiz":
		var q puzzle.Quiz
		_ = json.Unmarshal(p.Data, &q)
		for i, x := range q.Questions {
			fmt.Fprintf(&b, "%d. %s\n   → %s\n", i+1, x.Q, strings.Join(x.Answers, " / "))
			if x.Explain != "" {
				fmt.Fprintf(&b, "   (%s)\n", x.Explain)
			}
		}
	case "logic":
		var l puzzle.Logic
		_ = json.Unmarshal(p.Data, &l)
		for i, it := range l.Categories[0].Items {
			parts := []string{it}
			for k := 1; k < len(l.Categories); k++ {
				parts = append(parts, l.Categories[k].Items[l.Solution[k][i]])
			}
			b.WriteString(strings.Join(parts, " · ") + "\n")
		}
	case "ladder":
		var l puzzle.Ladder
		_ = json.Unmarshal(p.Data, &l)
		b.WriteString(strings.Join(l.Steps, " → ") + "\n")
	default:
		b.WriteString(puzzle.KindName[p.Kind] + ": the answer is the finished picture.\n")
	}
	return b.String()
}

func dirWord(dr, dc int) string {
	return map[[2]int]string{{0, 1}: "right", {0, -1}: "left", {1, 0}: "down", {-1, 0}: "up", {1, 1}: "down-right", {-1, -1}: "up-left", {1, -1}: "down-left", {-1, 1}: "up-right"}[[2]int{dr, dc}]
}

// ---- Pocket Tasks ----

// handlePrepTasks sends an adventure's open setting-up items to Pocket Tasks.
func (s *Server) handlePrepTasks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	sib, ok := s.sib(w, r, appTasks, scopeTasksWrite)
	if !ok {
		return
	}
	due := ""
	if a.Date != "" {
		due = store.AddDays(a.Date, -1)
		if due < today() {
			due = a.Date
		}
	}
	n := 0
	for i := range a.Prep {
		it := &a.Prep[i]
		if it.Done || it.TaskID != "" {
			continue
		}
		task, err := sib.client.CreateTask(ctx, clip(it.Text, 200), "Setting up “"+a.Title+"” (Pocket Puzzlecraft)", due, s.link(fmt.Sprintf("/adventures/%d?tab=prep", a.ID)))
		if err != nil {
			if n == 0 {
				s.appFail(w, appTasks, err)
				return
			}
			break
		}
		it.TaskID = strconv.FormatInt(task.ID, 10)
		it.TaskURL = linkTo(sib, "/tasks/"+it.TaskID)
		n++
	}
	_ = s.store.SaveAdventure(ctx, a)
	text := "Nothing new to send"
	if n > 0 {
		text = fmt.Sprintf("%d %s sent to Pocket Tasks", n, plural(n, "item", "items"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "sent": n, "text": text, "url": linkTo(sib, "/")})
}

// ---- Pocket Inventory ----

func (s *Server) handleInventory(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appInventory, scopeItemsRead)
	if !ok {
		return
	}
	q := url.Values{"limit": {"60"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/items", q)
	if err != nil {
		s.appFail(w, appInventory, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "items") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": num(m, "id"), "name": str(m, "name"), "location": str(m, "location_name"), "quantity": num(m, "quantity"), "url": linkTo(sib, "/items/"+idOf(m))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": out})
}

// ---- Pocket Places and Pocket Map ----

func (s *Server) handlePlaces(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appPlaces, scopePlacesRead)
	if !ok {
		return
	}
	q := url.Values{"limit": {"80"}}
	if t := strings.TrimSpace(r.URL.Query().Get("q")); t != "" {
		q.Set("q", t)
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/places", q)
	if err != nil {
		s.appFail(w, appPlaces, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "places") {
		out = append(out, map[string]any{"id": idOf(m), "name": str(m, "name"), "address": joinNonEmpty(", ", str(m, "address"), str(m, "city")), "lat": m["lat"], "lng": m["lng"],
			"kind": str(m, "kind"), "url": linkTo(sib, "/places/"+idOf(m))})
	}
	writeJSON(w, http.StatusOK, map[string]any{"places": out})
}

// handleToMap puts a hunt's stops on Pocket Map.
func (s *Server) handleToMap(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	a, err := s.store.GetAdventure(ctx, uid, idParam(r, "id"))
	if err != nil {
		s.fail(w, err, "adventure")
		return
	}
	sib, ok := s.sib(w, r, appMap, scopeMapWrite)
	if !ok {
		return
	}
	list := firstNonEmpty(s.settingsOf(ctx, uid).MapList, "Treasure hunts")
	n := 0
	for i, st := range a.Stages {
		if st.Lat == nil || st.Lng == nil {
			continue
		}
		body := map[string]any{"name": fmt.Sprintf("%s · %d. %s", a.Title, i+1, firstNonEmpty(st.PlaceName, st.Title)), "note": firstLine(st.Story), "kind": "flag", "list": list,
			"lat": *st.Lat, "lng": *st.Lng, "date": a.Date, "link_url": s.link(fmt.Sprintf("/adventures/%d", a.ID)), "source_ref": fmt.Sprintf("stage:%d", st.ID)}
		if _, err := postJSONTo(ctx, sib, "/api/v1/spots", body); err != nil {
			if n == 0 {
				s.appFail(w, appMap, err)
				return
			}
			break
		}
		n++
	}
	if n == 0 {
		writeErr(w, http.StatusBadRequest, "no stage has a place on the map yet")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": fmt.Sprintf("%d %s on Pocket Map", n, plural(n, "stop", "stops")), "url": linkTo(sib, "/")})
}

// ---- Pocket Learn ----

func (s *Server) handleLearnDecks(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appLearn, scopeLearnRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/decks", nil)
	if err != nil {
		s.appFail(w, appLearn, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "decks") {
		if m["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": idOf(m), "name": str(m, "name")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"decks": out})
}

// handleToLearn turns a puzzle's (or a word list's) words and clues into flashcards.
func (s *Server) handleToLearn(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		PuzzleID   int64  `json:"puzzle_id"`
		WordListID int64  `json:"wordlist_id"`
		Deck       string `json:"deck"`
	}
	if err := decode(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	pairs, ref := []store.WordClue{}, ""
	switch {
	case in.PuzzleID > 0:
		p, err := s.store.GetPuzzle(ctx, uid, in.PuzzleID)
		if err != nil {
			s.fail(w, err, "puzzle")
			return
		}
		pairs, ref = wordsOf(p), fmt.Sprintf("puzzle:%d", p.ID)
	case in.WordListID > 0:
		l, err := s.store.GetWordList(ctx, uid, in.WordListID)
		if err != nil {
			s.fail(w, err, "word list")
			return
		}
		pairs, ref = l.Words, fmt.Sprintf("wordlist:%d", l.ID)
	}
	cards := []store.WordClue{}
	for _, p := range pairs {
		if strings.TrimSpace(p.Clue) != "" {
			cards = append(cards, p)
		}
	}
	if len(cards) == 0 {
		writeErr(w, http.StatusBadRequest, "there are no words with clues to learn")
		return
	}
	sib, ok := s.sib(w, r, appLearn, scopeLearnWrite)
	if !ok {
		return
	}
	deck := firstNonEmpty(strings.TrimSpace(in.Deck), s.settingsOf(ctx, uid).LearnDeck, "Puzzle words")
	n, dup := 0, 0
	for _, c := range cards {
		res, err := postJSONTo(ctx, sib, "/api/v1/notes", map[string]any{"deck": deck, "kind": "basic", "front": c.Clue, "back": c.Word,
			"external_id": ref + ":" + strings.ToUpper(c.Word), "source_ref": ref, "source_url": s.link("/")})
		if err != nil {
			if n == 0 {
				s.appFail(w, appLearn, err)
				return
			}
			break
		}
		if res["duplicate"] == true {
			dup++
		} else {
			n++
		}
	}
	text := fmt.Sprintf("%d %s added to “%s” in Pocket Learn", n, plural(n, "card", "cards"), deck)
	if dup > 0 {
		text += fmt.Sprintf(" (%d already there)", dup)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": text, "url": linkTo(sib, "/")})
}

// wordsOf lists a puzzle's words with their clues.
func wordsOf(p *store.Puzzle) []store.WordClue {
	out := []store.WordClue{}
	switch p.Kind {
	case "crossword":
		c, _ := decodeCrossword(p.Data)
		for _, e := range puzzle.Entries(c) {
			if !strings.Contains(e.Answer, "?") {
				out = append(out, store.WordClue{Word: e.Answer, Clue: e.Clue})
			}
		}
	case "wordsearch":
		var ws puzzle.WordSearch
		_ = json.Unmarshal(p.Data, &ws)
		for _, x := range ws.Words {
			out = append(out, store.WordClue{Word: x.Word, Clue: x.Clue})
		}
	case "scramble":
		var s puzzle.Scramble
		_ = json.Unmarshal(p.Data, &s)
		for _, x := range s.Words {
			out = append(out, store.WordClue{Word: x.Word, Clue: x.Hint})
		}
	case "quiz":
		var q puzzle.Quiz
		_ = json.Unmarshal(p.Data, &q)
		for _, x := range q.Questions {
			if len(x.Answers) > 0 {
				out = append(out, store.WordClue{Word: x.Answers[0], Clue: x.Q})
			}
		}
	}
	return out
}

// ---- Pocket Bookshelf ----

func (s *Server) handleBooksList(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appBookshelf, scopeBooksRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/books", url.Values{"limit": {"200"}, "sort": {"recent"}})
	if err != nil {
		s.appFail(w, appBookshelf, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "books") {
		if num(m, "quote_count") <= 0 {
			continue
		}
		authors := ""
		if a, ok := m["authors"].([]any); ok && len(a) > 0 {
			authors, _ = a[0].(string)
		} else {
			authors = str(m, "authors")
		}
		out = append(out, map[string]any{"id": idOf(m), "title": str(m, "title"), "author": authors, "quotes": num(m, "quote_count")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"books": out})
}

func (s *Server) handleBookQuotes(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appBookshelf, scopeBooksRead)
	if !ok {
		return
	}
	id := chiParam(r, "id")
	if _, err := strconv.ParseInt(id, 10, 64); err != nil {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/books/"+id, nil)
	if err != nil {
		s.appFail(w, appBookshelf, err)
		return
	}
	m, _ := v.(map[string]any)
	book := obj(m, "book")
	author := ""
	if a, ok := book["authors"].([]any); ok && len(a) > 0 {
		author, _ = a[0].(string)
	}
	out := []map[string]any{}
	for _, q := range rows(m["quotes"]) {
		if q["hidden"] == true || len(str(q, "text")) > 600 {
			continue
		}
		out = append(out, map[string]any{"text": str(q, "text"), "author": author, "book": str(book, "title")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"quotes": out})
}

// ---- Pocket Habits ----

func (s *Server) handleHabits(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appHabits, scopeHabitsRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/habits", nil)
	if err != nil {
		s.appFail(w, appHabits, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "habits") {
		out = append(out, map[string]any{"id": num(m, "id"), "name": str(m, "name"), "color": str(m, "color")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"habits": out})
}

// tickHabit marks the person's puzzle habit done today (when they chose one).
func (s *Server) tickHabit(ctx context.Context, uid int64) {
	st := s.settingsOf(ctx, uid)
	if st.HabitID <= 0 || !s.once(ctx, fmt.Sprintf("habit:%d:%s", uid, today())) {
		return
	}
	sib, err := s.clients(ctx, uid).conn(ctx, appHabits, scopeEntriesWrite)
	if err != nil {
		return
	}
	var out map[string]any
	if err := sib.client.Do(ctx, http.MethodPut, fmt.Sprintf("/api/v1/habits/%d/entries/%s", st.HabitID, today()), nil, map[string]any{"status": 1}, &out); err != nil {
		s.log.Debug("habit tick failed", "err", err)
	}
}

// ---- Pocket Quest ----

func (s *Server) handleQuestCampaigns(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appQuest, scopeQuestRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/campaigns", nil)
	if err != nil {
		s.appFail(w, appQuest, err)
		return
	}
	out := []map[string]any{}
	for _, m := range rows(v, "campaigns") {
		if str(m, "status") == "archived" {
			continue
		}
		out = append(out, map[string]any{"id": num(m, "id"), "name": str(m, "name"), "next_session": m["next_session"]})
	}
	writeJSON(w, http.StatusOK, map[string]any{"campaigns": out})
}

// handleToQuest hands a puzzle to tabletop players: a handout in a campaign.
func (s *Server) handleToQuest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	var in struct {
		What       string `json:"what"`
		ID         int64  `json:"id"`
		CampaignID int64  `json:"campaign_id"`
	}
	if err := decode(r, &in); err != nil || in.CampaignID <= 0 {
		writeErr(w, http.StatusBadRequest, "pick a campaign")
		return
	}
	sh, err := s.shareableOf(ctx, uid, firstNonEmpty(in.What, "puzzle"), in.ID)
	if err != nil {
		s.fail(w, err, "puzzle")
		return
	}
	sib, ok := s.sib(w, r, appQuest, scopeQuestWrite)
	if !ok {
		return
	}
	body := fmt.Sprintf("%s\n\n%s", sh.sub, sh.url)
	secret := ""
	if in.What == "puzzle" || in.What == "" {
		if p, err := s.store.GetPuzzle(ctx, uid, in.ID); err == nil {
			secret = "Answer:\n" + answerKey(p)
			if !p.ShareOn {
				body = sh.sub + "\n\n(Switch on the puzzle's share link in Pocket Puzzlecraft so players can open it.)"
			}
		}
	}
	res, err := postJSONTo(ctx, sib, "/api/v1/codex", map[string]any{"campaign_id": in.CampaignID, "kind": "handout", "name": clip(sh.title, 120), "subtitle": "A puzzle", "body": body,
		"secret": secret, "tags": []string{"puzzle"}})
	if err != nil {
		s.appFail(w, appQuest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Added to the campaign in Pocket Quest", "url": linkTo(sib, fmt.Sprintf("/campaigns/%d?tab=codex&entry=%s", in.CampaignID, idOf(res)))})
}

// ---- Pocket Blog ----

func (s *Server) handleToBlog(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := actorID(ctx)
	in, sh, ok := s.sendTarget(w, r)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appBlog, scopeBlogWrite)
	if !ok {
		return
	}
	intro := firstNonEmpty(strings.TrimSpace(in.Text), "I made a puzzle — have a go!")
	body := fmt.Sprintf("%s\n\n**%s** — %s\n\n[Play it here](%s) (no account needed).\n", intro, sh.title, sh.sub, sh.url)
	res, err := postJSONTo(ctx, sib, "/api/v1/posts", map[string]any{"title": clip(sh.title, 200), "body": body, "excerpt": clip(sh.sub, 200), "tags": []string{"puzzles"},
		"source_ref": fmt.Sprintf("%s:%d", in.What, in.ID)})
	if err != nil {
		s.appFail(w, appBlog, err)
		return
	}
	_ = uid
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "A draft post is waiting in Pocket Blog", "url": linkTo(sib, "/posts/"+idOf(res))})
}

// ---- Pocket Family ----

func (s *Server) handleFamily(w http.ResponseWriter, r *http.Request) {
	sib, ok := s.sib(w, r, appFamily, scopeFamilyRead)
	if !ok {
		return
	}
	v, err := getJSONFrom(r.Context(), sib, "/api/v1/family", nil)
	if err != nil {
		s.appFail(w, appFamily, err)
		return
	}
	m, _ := v.(map[string]any)
	out := []map[string]any{}
	for _, p := range rows(m["people"]) {
		if p["archived"] == true {
			continue
		}
		out = append(out, map[string]any{"id": num(p, "id"), "name": str(p, "name"), "role": str(p, "role"), "birthday": str(p, "birthday"), "emoji": str(p, "emoji"), "email": str(p, "email")})
	}
	writeJSON(w, http.StatusOK, map[string]any{"family": str(m, "name"), "people": out})
}

// handleToFamily pins a puzzle to the family board.
func (s *Server) handleToFamily(w http.ResponseWriter, r *http.Request) {
	in, sh, ok := s.sendTarget(w, r)
	if !ok {
		return
	}
	sib, ok := s.sib(w, r, appFamily, scopeFamilyWrite)
	if !ok {
		return
	}
	text := firstNonEmpty(strings.TrimSpace(in.Text), "New puzzle! Who can solve it first?")
	body := fmt.Sprintf("%s %s — %s\n%s", sh.icon, text, sh.title, sh.url)
	if _, err := postJSONTo(r.Context(), sib, "/api/v1/posts", map[string]any{"body": clip(body, 2000), "until": store.AddDays(today(), 7)}); err != nil {
		s.appFail(w, appFamily, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "text": "Pinned to the family board", "url": linkTo(sib, "/board")})
}

// ---- Pocket Translate ----

func (s *Server) handleTranslate(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Texts []string `json:"texts"`
		To    string   `json:"to"`
		From  string   `json:"from"`
	}
	if err := decode(r, &in); err != nil || len(in.Texts) == 0 || len(in.Texts) > 200 || strings.TrimSpace(in.To) == "" {
		writeErr(w, http.StatusBadRequest, "send up to 200 texts and the language to translate to")
		return
	}
	sib, ok := s.sib(w, r, appTranslate, scopeTranslate)
	if !ok {
		return
	}
	res, err := postJSONTo(r.Context(), sib, "/api/v1/translate", map[string]any{"texts": in.Texts, "to": in.To, "from": firstNonEmpty(in.From, "auto")})
	if err != nil {
		s.appFail(w, appTranslate, err)
		return
	}
	out := []string{}
	if arr, ok := res["texts"].([]any); ok {
		for _, x := range arr {
			v, _ := x.(string)
			out = append(out, v)
		}
	}
	if len(out) != len(in.Texts) {
		writeErr(w, http.StatusBadGateway, "Pocket Translate gave back a different number of texts")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"texts": out})
}

// ---- Pocket Alerts ----

func (s *Server) alert(ctx context.Context, uid int64, title, link string) {
	sib, err := s.clients(ctx, uid).conn(ctx, appAlerts, scopeAlertsWrite)
	if err != nil {
		return
	}
	if _, err := postJSONTo(ctx, sib, "/api/v1/alerts", map[string]any{"title": clip(title, 200), "body": "Pocket Puzzlecraft", "priority": "low", "tags": []string{"jigsaw"}, "click": link}); err != nil {
		s.log.Debug("alert failed", "err", err)
	}
}

func htmlEsc(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}
