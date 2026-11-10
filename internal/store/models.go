package store

import (
	"encoding/json"
	"strings"
)

// User is an account on this box.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"` // admin | member
	CreatedAt    string `json:"created_at"`
}

// APIToken is a personal access token (hash stored, plaintext shown once).
type APIToken struct {
	ID         int64   `json:"id"`
	UserID     int64   `json:"user_id"`
	Name       string  `json:"name"`
	Prefix     string  `json:"prefix"`
	LastUsedAt *string `json:"last_used_at"`
	CreatedAt  string  `json:"created_at"`
}

// Photo is a picture: a room, a floor plan to trace over, a piece of furniture.
type Photo struct {
	ID        int64  `json:"id"`
	UserID    int64  `json:"-"`
	Kind      string `json:"kind"`
	Mime      string `json:"mime"`
	Data      []byte `json:"-"`
	Thumb     []byte `json:"-"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	Bytes     int    `json:"bytes"`
	CreatedAt string `json:"created_at"`
}

// Puzzle is one puzzle of any kind; Data is its definition with the answer.
type Puzzle struct {
	ID           int64           `json:"id"`
	UserID       int64           `json:"-"`
	Kind         string          `json:"kind"`
	Title        string          `json:"title"`
	Instructions string          `json:"instructions"`
	Notes        string          `json:"notes"`
	Level        string          `json:"level"`
	Language     string          `json:"language"`
	Tags         []string        `json:"tags"`
	Data         json.RawMessage `json:"data,omitempty"`
	SizeText     string          `json:"size_text"`
	Ready        bool            `json:"ready"`
	Problems     int             `json:"problems"`
	TestedAt     *string         `json:"tested_at"`
	PhotoID      *int64          `json:"photo_id"`
	ShareToken   string          `json:"share_token"`
	ShareOn      bool            `json:"share_on"`
	Board        bool            `json:"board"`
	Reveal       bool            `json:"reveal"`
	Timer        bool            `json:"timer"`
	Source       string          `json:"source"`
	SourceRef    string          `json:"source_ref"`
	Pinned       bool            `json:"pinned"`
	Archived     bool            `json:"archived"`
	CreatedAt    string          `json:"created_at"`
	UpdatedAt    string          `json:"updated_at"`

	// worked out on read
	Plays  int    `json:"plays"`
	Solves int    `json:"solves"`
	BestMs int64  `json:"best_ms"`
	Mine   *Play  `json:"mine,omitempty"` // your own go, when there is one
	InUse  string `json:"in_use,omitempty"`
}

// Book is a puzzle book.
type Book struct {
	ID         int64      `json:"id"`
	UserID     int64      `json:"-"`
	Title      string     `json:"title"`
	Subtitle   string     `json:"subtitle"`
	Notes      string     `json:"notes"`
	Answers    string     `json:"answers"`
	Large      bool       `json:"large"`
	ShareToken string     `json:"share_token"`
	ShareOn    bool       `json:"share_on"`
	CreatedAt  string     `json:"created_at"`
	UpdatedAt  string     `json:"updated_at"`
	Items      []BookItem `json:"items"`
}

// BookItem is a puzzle in a book.
type BookItem struct {
	ID       int64  `json:"id"`
	PuzzleID int64  `json:"puzzle_id"`
	Sort     int    `json:"sort"`
	Title    string `json:"title"`
	Kind     string `json:"kind"`
	Level    string `json:"level"`
	SizeText string `json:"size_text"`
}

// PrepItem is one thing to do before an adventure (hide the clues, buy padlocks).
type PrepItem struct {
	ID      string `json:"id"`
	Text    string `json:"text"`
	Done    bool   `json:"done"`
	TaskURL string `json:"task_url,omitempty"`
	TaskID  string `json:"task_id,omitempty"`
}

// Adventure is an escape room, a treasure hunt or a mystery.
type Adventure struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"-"`
	Kind        string     `json:"kind"`
	Title       string     `json:"title"`
	Intro       string     `json:"intro"`
	Finale      string     `json:"finale"`
	Theme       string     `json:"theme"`
	Date        string     `json:"date"`
	Time        string     `json:"time"`
	Place       string     `json:"place"`
	Players     string     `json:"players"`
	Minutes     int        `json:"minutes"`
	HintPenalty int        `json:"hint_penalty"`
	Status      string     `json:"status"`
	Notes       string     `json:"notes"`
	Prep        []PrepItem `json:"prep"`
	ShareToken  string     `json:"share_token"`
	ShareOn     bool       `json:"share_on"`
	Board       bool       `json:"board"`
	CalendarRef string     `json:"calendar_ref"`
	CalendarURL string     `json:"calendar_url"`
	PhotoID     *int64     `json:"photo_id"`
	Source      string     `json:"source"`
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`

	Stages  []Stage `json:"stages,omitempty"`
	StagesN int     `json:"stages_n"`
	Plays   int     `json:"plays"`
	Live    int     `json:"live"` // teams playing right now
}

// Hint is a hint at a stage, offered after some minutes there.
type Hint struct {
	Text  string `json:"text"`
	After int    `json:"after"`
}

// Stage is one step of an adventure.
type Stage struct {
	ID          int64    `json:"id"`
	UserID      int64    `json:"-"`
	AdventureID int64    `json:"adventure_id"`
	Sort        int      `json:"sort"`
	Title       string   `json:"title"`
	Story       string   `json:"story"`
	PhotoID     *int64   `json:"photo_id"`
	Lock        string   `json:"lock"`
	PuzzleID    *int64   `json:"puzzle_id"`
	Answers     []string `json:"answers"`
	LockHint    string   `json:"lock_hint"`
	Hints       []Hint   `json:"hints"`
	Hidden      string   `json:"hidden"`
	PlaceName   string   `json:"place_name"`
	Lat         *float64 `json:"lat"`
	Lng         *float64 `json:"lng"`
	PlaceRef    string   `json:"place_ref"`
	Unlock      string   `json:"unlock"`
	Props       string   `json:"props"`
	Code        string   `json:"code"`
	CreatedAt   string   `json:"created_at"`

	PuzzleTitle string `json:"puzzle_title,omitempty"`
	PuzzleKind  string `json:"puzzle_kind,omitempty"`
}

// Play is one go at something.
type Play struct {
	ID         int64           `json:"id"`
	UserID     int64           `json:"-"`
	What       string          `json:"what"`
	RefID      int64           `json:"ref_id"`
	Kind       string          `json:"kind"`
	Day        string          `json:"day"`
	Via        string          `json:"via"`
	Player     string          `json:"player"`
	Key        string          `json:"-"`
	StartedAt  string          `json:"started_at"`
	FinishedAt *string         `json:"finished_at"`
	Ms         int64           `json:"ms"`
	Hints      int             `json:"hints"`
	Mistakes   int             `json:"mistakes"`
	Score      int             `json:"score"`
	MaxScore   int             `json:"max_score"`
	Stage      int             `json:"stage"`
	StageAt    *string         `json:"stage_at"`
	State      json.RawMessage `json:"state"`
	LastAt     string          `json:"last_at"`

	Title string `json:"title,omitempty"` // what was played, for lists
}

// WordList is a list of words with clues.
type WordList struct {
	ID        int64      `json:"id"`
	UserID    int64      `json:"-"`
	Name      string     `json:"name"`
	Language  string     `json:"language"`
	Words     []WordClue `json:"words"`
	Source    string     `json:"source"`
	CreatedAt string     `json:"created_at"`
	UpdatedAt string     `json:"updated_at"`
	Count     int        `json:"count"`
}

// WordClue is one word of a list.
type WordClue struct {
	Word string `json:"word"`
	Clue string `json:"clue"`
}

// Allowed values.
var (
	PhotoKinds     = []string{"puzzle", "stage", "cover"}
	AdventureKinds = []string{"escape", "hunt", "mystery", "party"}
	AdventureState = []string{"draft", "ready", "done"}
	Locks          = []string{"answer", "code", "word", "directions", "colors", "qr", "puzzle", "none"}
)

// Conn is one person's connection to a sibling Pocket app.
type Conn struct {
	Mode      string `json:"mode,omitempty"`
	URL       string `json:"url,omitempty"`
	Token     string `json:"token,omitempty"`
	PublicURL string `json:"public_url,omitempty"`
}

// Settings are the box-wide values plus one person's preferences and their
// own connections.
type Settings struct {
	Title         string `json:"title"`
	SetupComplete bool   `json:"setup_complete"`

	DateFormat string `json:"date_format"`
	TimeFormat string `json:"time_format"`
	WeekStart  int    `json:"week_start"` // 1 = Monday, 7 = Sunday

	// Making and playing.
	Language    string `json:"language"`     // a new puzzle's language: en | bcs | other
	Level       string `json:"level"`        // a new puzzle's difficulty
	PaperSize   string `json:"paper_size"`   // a4 | letter
	LargePrint  bool   `json:"large_print"`  // print bigger squares and letters
	ShowMistake bool   `json:"show_mistake"` // while you play: mark wrong letters at once
	Daily       bool   `json:"daily"`        // the puzzles of the day on the Today page
	DailyLevel  string `json:"daily_level"`  // "" = changes with the weekday
	ShareBoard  bool   `json:"share_board"`  // new share links show the best times

	// Other apps.
	NotesFolder string `json:"notes_folder"` // where answer keys and host sheets go in Pocket Notes
	FilesFolder string `json:"files_folder"` // where printouts and exports go in Pocket Files
	ChatTo      string `json:"chat_to"`      // the Pocket Chat conversation to share puzzles in
	HabitID     int64  `json:"habit_id"`     // the Pocket Habits habit ticked when you solve the daily puzzle
	LearnDeck   string `json:"learn_deck"`   // the Pocket Learn deck words go to
	MapList     string `json:"map_list"`     // the Pocket Map list hunt stops go on
	AlertPlays  bool   `json:"alert_plays"`  // Pocket Alerts: someone finished your puzzle or adventure
	AIHelp      bool   `json:"ai_help"`
	AIPhotos    bool   `json:"ai_photos"` // the AI may read a photo of a printed puzzle

	// Notifications.
	NotifyPlays  bool   `json:"notify_plays"` // someone finished a puzzle you shared
	NotifyDaily  bool   `json:"notify_daily"` // a morning message: today's puzzles are ready
	NotifyPrep   bool   `json:"notify_prep"`  // the day before an adventure: what is still to set up
	NotifyHour   int    `json:"notify_hour"`
	NotifyURL    string `json:"notify_url"`
	NotifyFormat string `json:"notify_format"` // apprise | ntfy | json

	// Each sibling Pocket app, by slug.
	Conns map[string]Conn `json:"conns"`

	// AI: Pocket AI through Core, or an OpenAI-compatible API with the
	// person's own key.
	AIProvider    string `json:"ai_provider"` // auto | pocket | openai
	AIPocketModel string `json:"ai_pocket_model"`
	OpenAIKey     string `json:"openai_api_key"`
	OpenAIModel   string `json:"openai_model"`
	OpenAIBaseURL string `json:"openai_base_url"`
}

func toJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// CleanTags trims, de-duplicates and caps a list of tags.
func CleanTags(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, t := range in {
		t = strings.TrimSpace(strings.TrimPrefix(t, "#"))
		k := strings.ToLower(t)
		if t == "" || seen[k] || len(out) >= 20 {
			continue
		}
		seen[k] = true
		out = append(out, t)
	}
	return out
}
