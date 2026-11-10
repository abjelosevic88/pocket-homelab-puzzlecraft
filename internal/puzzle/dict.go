// Package puzzle makes, checks and solves the puzzles: crosswords, word
// searches, sudoku, picture puzzles (nonograms), secret codes, word
// scrambles, quizzes, mazes, logic grids, word ladders and jigsaws.
//
// Everything here is pure: no database, no HTTP. The server stores a
// puzzle's definition (with its answer) as JSON; Public strips the answer
// for players, Check marks what a player sent, Test says whether a puzzle
// is fair (one answer, nothing missing) and Generate makes one.
package puzzle

import (
	"bufio"
	"bytes"
	"compress/gzip"
	_ "embed"
	"sort"
	"strings"
	"sync"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// The English word list is ENABLE (public domain); "common" is the
// popular.txt subset from github.com/dolph/dictionary, used to prefer
// everyday words when filling grids and making ladders.
//
//go:embed data/enable.txt.gz
var enableGz []byte

//go:embed data/common.txt.gz
var commonGz []byte

type dictionary struct {
	all    map[string]bool
	common map[string]bool
	byLen  map[int][]string // common words first, then the rest
}

var (
	dictOnce sync.Once
	dict     *dictionary
)

func readGz(b []byte) []string {
	zr, err := gzip.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil
	}
	defer zr.Close()
	var out []string
	sc := bufio.NewScanner(zr)
	for sc.Scan() {
		if w := strings.TrimSpace(sc.Text()); w != "" {
			out = append(out, strings.ToUpper(w))
		}
	}
	return out
}

func words() *dictionary {
	dictOnce.Do(func() {
		d := &dictionary{all: map[string]bool{}, common: map[string]bool{}, byLen: map[int][]string{}}
		for _, w := range readGz(commonGz) {
			d.common[w] = true
		}
		for _, w := range readGz(enableGz) {
			d.all[w] = true
		}
		for w := range d.common {
			d.all[w] = true
		}
		rest := map[int][]string{}
		for w := range d.all {
			n := len(w)
			if d.common[w] {
				d.byLen[n] = append(d.byLen[n], w)
			} else {
				rest[n] = append(rest[n], w)
			}
		}
		for n := range d.byLen {
			sort.Strings(d.byLen[n])
		}
		for n, ws := range rest {
			sort.Strings(ws)
			d.byLen[n] = append(d.byLen[n], ws...)
		}
		dict = d
	})
	return dict
}

// IsWord says whether an English word is in the word list.
func IsWord(w string) bool { return words().all[strings.ToUpper(w)] }

// IsCommon says whether it is an everyday English word.
func IsCommon(w string) bool { return words().common[strings.ToUpper(w)] }

// Match lists English words fitting a pattern ("C?T", "?" or "." = any
// letter), everyday words first, at most limit.
func Match(pattern string, limit int, commonOnly bool) []string {
	p := []rune(strings.ToUpper(strings.TrimSpace(pattern)))
	if len(p) == 0 || len(p) > 21 {
		return []string{}
	}
	out := []string{}
	for _, w := range words().byLen[len(p)] {
		if commonOnly && !words().common[w] {
			break
		}
		if fits(w, p) {
			out = append(out, w)
			if len(out) >= limit {
				break
			}
		}
	}
	return out
}

func fits(w string, p []rune) bool {
	for i, r := range w {
		if c := p[i]; c != '?' && c != '.' && c != '_' && c != ' ' && c != r {
			return false
		}
	}
	return true
}

// ---- letters ----

// Languages a puzzle can be in. "bcs" is Bosnian/Croatian/Serbian in Latin
// letters, where LJ, NJ and DŽ are one letter (one square).
var Languages = []string{"en", "bcs", "other"}

var bcsAlphabet = []string{"A", "B", "C", "Č", "Ć", "D", "DŽ", "Đ", "E", "F", "G", "H", "I", "J", "K", "L", "LJ", "M", "N", "NJ", "O", "P", "R", "S", "Š", "T", "U", "V", "Z", "Ž"}
var enAlphabet = strings.Split("ABCDEFGHIJKLMNOPQRSTUVWXYZ", "")

// Alphabet is the language's letters (for filling word searches and codes).
func Alphabet(lang string) []string {
	if lang == "bcs" {
		return bcsAlphabet
	}
	return enAlphabet
}

// Letters splits a word into its letters, upper case, without spaces or
// punctuation; in "bcs" LJ, NJ and DŽ stay one letter.
func Letters(word, lang string) []string {
	rs := []rune(strings.ToUpper(norm.NFC.String(word)))
	out := []string{}
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		if lang == "bcs" && i+1 < len(rs) {
			pair := string([]rune{r, rs[i+1]})
			if pair == "LJ" || pair == "NJ" || pair == "DŽ" {
				out = append(out, pair)
				i++
				continue
			}
		}
		out = append(out, string(r))
	}
	return out
}

// Fold is a letter or word without accents, for forgiving comparisons.
func Fold(s string) string {
	s = strings.ReplaceAll(strings.ReplaceAll(s, "Đ", "DJ"), "đ", "dj")
	d := norm.NFD.String(s)
	var b strings.Builder
	for _, r := range d {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Answer normalises a typed answer: lower case, single spaces, no
// punctuation at the ends.
func Answer(s string) string {
	s = strings.ToLower(norm.NFC.String(strings.TrimSpace(s)))
	var b strings.Builder
	space := false
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			if space && b.Len() > 0 {
				b.WriteByte(' ')
			}
			space = false
			b.WriteRune(r)
		default:
			space = true
		}
	}
	return b.String()
}

// SameAnswer compares a typed answer with an accepted one: case, spaces,
// punctuation and accents do not matter, nor a leading "a"/"an"/"the".
func SameAnswer(typed, want string) bool {
	a, b := Answer(typed), Answer(want)
	if a == "" || b == "" {
		return false
	}
	if a == b || Fold(a) == Fold(b) {
		return true
	}
	strip := func(s string) string {
		for _, p := range []string{"the ", "a ", "an "} {
			s = strings.TrimPrefix(s, p)
		}
		return strings.ReplaceAll(Fold(s), " ", "")
	}
	return strip(a) == strip(b)
}

func upper(s string) string { return strings.ToUpper(norm.NFC.String(strings.TrimSpace(s))) }
