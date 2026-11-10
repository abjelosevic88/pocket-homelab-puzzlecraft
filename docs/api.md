# API

Everything the web app does goes through `/api/v1`. Authenticate with a session cookie, a personal token (`Authorization: Bearer ppzc_…`, Settings → API & shortcuts) or a Pocket Core link token. Every row belongs to one person; other people's ids answer 404.

## Overview

| | |
|---|---|
| `GET /today` (`?date=`) | The puzzles of the day (sudoku, word ladder, one that changes with the weekday) and how far you are, your run of days, what you are in the middle of, escape rooms coming up, who solved what you shared |
| `GET /summary` | Counts by kind, shared puzzles, escape rooms coming up, today's puzzles solved, the run of days — for Homepage and Pocket AI |
| `GET /stats` | Solved per kind with usual and best times, the last 84 days |
| `GET /day/{YYYY-MM-DD}` | What was solved that day (for Pocket Day) and adventures planned |
| `GET /search?q=` | Puzzles, adventures, books, word lists |
| `GET /kinds`, `GET /themes`, `GET /templates` | Kinds, levels, languages; built-in word lists, pictures, logic sets, quotes; ready-made adventures |

## Puzzles

| | |
|---|---|
| `GET /puzzles` (`?kind=&q=&tag=&shared=1&archived=1`) | List (no definitions), counts by kind, tags |
| `POST /puzzles` | `{"kind", "title", "level", "language", "data"}` or `{"kind", "generate": {…Options}}` or `{"kind", "words": [{"word","clue"}]}` |
| `GET/PUT/DELETE /puzzles/{id}` | One puzzle with its definition (`data`, answer included — other apps get the player's view) |
| `POST /puzzles/{id}/test` | Is it fair: one answer, nothing missing → `{ok, problems, notes, level}` |
| `POST /puzzles/{id}/share` | `{"on", "board", "reveal", "timer", "renew"}` — the link is `/p/{share_token}` |
| `POST /puzzles/{id}/duplicate`, `GET /puzzles/{id}/plays`, `GET /puzzles/{id}/print` | |
| `GET /puzzles/{id}/export.ipuz`, `/export.puz` | Crosswords (and sudoku for .ipuz) as files |
| `POST /generate` | `{"kind", …Options}` → a puzzle made, not saved (`internal/puzzle/generate.go` lists the options) |
| `POST /test` | `{"kind", "data"}` → the test of an unsaved definition |
| `GET /words?pattern=C?T` | Words that fit (everyday first, then your word lists) |
| `POST /crossword/grid`, `POST /crossword/fill` | A blank symmetric grid; fill a grid's empty squares with words |

Definitions per kind are the Go structs in `internal/puzzle/types.go`.

## Playing

A go is found by its key (`play_key`); the answer never leaves the server.

| | |
|---|---|
| `POST /puzzles/{id}/play`, `POST /daily/{date}/{kind}/play` | Your own go (resumes) |
| `GET /public/p/{token}`, `POST /public/p/{token}/play` | A shared puzzle; `{"player", "key"}` starts or carries on |
| `GET /public/b/{token}`, `POST /public/b/{token}/play` | A shared book; `{"puzzle_id", "player", "key"}` |
| `GET /public/a/{token}`, `POST /public/a/{token}/play` | A shared escape room / hunt |
| `GET /public/play/{key}` | Where a go stands |
| `POST /public/play/{key}/save` | `{"progress"}` |
| `POST /public/play/{key}/check` | `{"answer", "final", "ms"}` → `{result: {solved, wrong, right, …}}` |
| `POST /public/play/{key}/reveal` | `{"what"}` → one square / word / step (a hint) |
| `POST /public/play/{key}/answer` | An adventure stage's lock: `{"answer"}` |
| `POST /public/play/{key}/scan` | A QR card's `{"code"}` |
| `POST /public/play/{key}/hint` | The stage's next hint, once its minutes have passed |

## Books, word lists, adventures

`GET/POST /books`, `GET/PUT/DELETE /books/{id}` (`{"title", "subtitle", "answers": "back|none", "large", "share_on", "puzzle_ids"}`), `GET /books/{id}/print` ·
`GET/POST /wordlists`, `GET/PUT/DELETE /wordlists/{id}` (`{"name", "language", "words"}` or `{"text": "word - clue\n…"}`) ·
`GET/POST /adventures` (`{"template": "study|birthday|hunt|kids"}` or fields), `GET/PUT/DELETE /adventures/{id}`, `POST /adventures/{id}/stages`, `PUT /stages/{id}`, `DELETE /stages/{id}`, `PUT /adventures/{id}/order`, `POST /adventures/{id}/play` (a test run), `GET /adventures/{id}/live`, `GET /adventures/{id}/plays`.

## Other Pocket apps

`GET /apps` (how you reach each), `GET /apps/chats`, `POST /apps/chat`, `POST /apps/email`, `GET /apps/contacts`, `POST /apps/calendar`, `GET /apps/photos`, `POST /apps/photos/{uuid}/copy`, `POST /apps/files`, `POST /apps/notes`, `POST /adventures/{id}/tasks`, `GET /apps/inventory`, `GET /apps/places`, `POST /adventures/{id}/map`, `GET /apps/learn/decks`, `POST /apps/learn`, `GET /apps/books`, `GET /apps/books/{id}/quotes`, `GET /apps/habits`, `GET /apps/quest/campaigns`, `POST /apps/quest`, `POST /apps/blog`, `GET /apps/family`, `POST /apps/family`, `POST /apps/translate`.

AI (Pocket AI or your own key): `POST /ai/clues`, `/ai/words`, `/ai/quiz`, `/ai/story`, `/ai/hints`, `/ai/check-clue`, `/ai/read-puzzle`.

## Pocket Core links

With a `pcx_` token from Pocket Core another app reaches only: `puzzlecraft:read` — `GET summary, home, search, stats, today, kinds, themes, puzzles, books, adventures, wordlists, words, day/{date}` and the single items and their plays (definitions as players see them; adventures without answers, hiding places or notes); `puzzlecraft:write` — `POST puzzles, wordlists, generate`, `PUT/DELETE puzzles/{id}, wordlists/{id}` (only what that app added), `POST puzzles/{id}/share|test`.

Events on the Core bus: `puzzlecraft.puzzle_created`, `puzzle_shared`, `puzzle_solved`, `daily_solved`, `adventure_created`, `adventure_finished`.

## Data

`GET /export.json` (`?photos=0` without pictures) · `GET /export.csv` · `POST /import` (a Puzzlecraft export — again and again, nothing twice; a .puz or .ipuz file; a sudoku as 81 digits) · `GET /backup` (admins: the whole database).
