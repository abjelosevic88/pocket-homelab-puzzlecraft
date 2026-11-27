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

## Pocket Search, Pocket Day and Pocket Hub

Puzzlecraft answers the four routes every Pocket app can offer (`docs/capabilities.md` in Pocket Core), so Pocket Search, Pocket Day and Pocket Hub find it on their own. Each has its own link scope that opens that one route and nothing else (`puzzlecraft:read` and `puzzlecraft:write` do not open them); a signed-in person or a personal token can call them too. Addresses (`url`) are paths in the app. No answer, hiding place or private note is in any of them.

| Route | Scope | What |
|---|---|---|
| `GET /pocket/search?q&limit` | `pocket:search` | `{items}`: your puzzles, escape rooms and hunts, puzzle books and word lists matching `q`, a title or name that has the words first, the kinds taking turns. Each has `id` (`puzzle:3`, `adventure:1`, `book:2`, `wordlist:4`), `title`, `snippet` (size and level; the place and the first line of the story; how many puzzles or words), `kind` (crossword, word search, sudoku, … escape room, treasure hunt, mystery, party game, puzzle book, word list), `date` and `url`. What somebody else shared with you by link is never there |
| `GET /pocket/day/{date}` (`?tz=`) | `pocket:day` | `{date, summary, items, metrics}`: the puzzles you made that day (`kind` made), what you solved with its `minutes` (solved; a test run of your own escape room is tested), what players of your shared puzzles and games finished (solved by others) and the escape rooms and hunts set for that day (planned). `summary` is “2 puzzles solved in 9 min, 1 puzzle made, 1 escape room planned”. The day is the one of `tz` when given, else the box's |
| `GET /pocket/widget` | `pocket:widget` | `{line, metrics, items, url}`: the puzzles of the day (`done` when solved, `detail` “Solved in 4:05”, “Started” or “Not solved yet”), then the puzzles you made or changed last. `line` is “Today's 3 puzzles are waiting” or “1 of 3 solved today, 5 days in a row”; the numbers are solved today, days in a row, the longest run and how many puzzles you have. Reading it makes no puzzle |
| `POST /pocket/capture` | `pocket:capture` | `{request_id, kind: "words", text, title}` adds a word list and answers `201 {id, title, url}`. The words are one per line or with commas between them (“apple, pear, plum”; a line “plum - a stone fruit” keeps its clue); `title` is the list's name, without one it is named after its first words (“Apple, pear, plum”). Fewer than two words of two letters or more answers `400`. The same `request_id` again answers the same and adds nothing |

## Data

`GET /export.json` (`?photos=0` without pictures) · `GET /export.csv` · `POST /import` (a Puzzlecraft export — again and again, nothing twice; a .puz or .ipuz file; a sudoku as 81 digits) · `GET /backup` (admins: the whole database).

Pocket Move can copy your puzzles, books, escape rooms and word lists out and bring them in: `GET /pocket/portable` describes the exports (link scope `move:export`) and the import (`move:import`).
