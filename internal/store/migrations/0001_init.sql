CREATE TABLE users (
  id            INTEGER PRIMARY KEY AUTOINCREMENT,
  email         TEXT NOT NULL UNIQUE,
  name          TEXT NOT NULL DEFAULT '',
  password_hash TEXT NOT NULL DEFAULT '',
  role          TEXT NOT NULL DEFAULT 'member',
  created_at    TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at    TEXT
);

CREATE TABLE sessions (
  id         TEXT PRIMARY KEY,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  expires_at TEXT NOT NULL,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE api_tokens (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name         TEXT NOT NULL DEFAULT '',
  token_hash   TEXT NOT NULL UNIQUE,
  prefix       TEXT NOT NULL DEFAULT '',
  last_used_at TEXT,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

-- Box-wide settings (title, setup flag); everything else is personal.
CREATE TABLE settings (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

CREATE TABLE user_settings (
  user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  key     TEXT NOT NULL,
  value   TEXT NOT NULL DEFAULT '',
  PRIMARY KEY (user_id, key)
);

CREATE TABLE cache (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  fetched_at TEXT NOT NULL
);


-- ---------- Pocket Puzzlecraft: everything below is per person ----------
-- A puzzle's definition, answer included, is JSON in puzzles.data (see
-- internal/puzzle): players only ever get the part without the answer.
-- Times are milliseconds, dates YYYY-MM-DD.

-- Pictures: a jigsaw, a quiz question, an escape-room stage, a cover.
-- The browser shrinks them first (2000 px and a 480 px thumbnail, JPEG).
CREATE TABLE photos (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind       TEXT NOT NULL DEFAULT 'puzzle',        -- puzzle | stage | cover
  mime       TEXT NOT NULL DEFAULT 'image/jpeg',
  data       BLOB NOT NULL,
  thumb      BLOB,
  width      INTEGER NOT NULL DEFAULT 0,
  height     INTEGER NOT NULL DEFAULT 0,
  bytes      INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX photos_user ON photos(user_id, kind);

-- A puzzle of any kind: crossword, word search, sudoku, picture puzzle,
-- secret code, scramble, quiz, maze, logic grid, word ladder, jigsaw.
CREATE TABLE puzzles (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL,
  title        TEXT NOT NULL,
  instructions TEXT NOT NULL DEFAULT '',             -- shown to players
  notes        TEXT NOT NULL DEFAULT '',             -- yours only
  level        TEXT NOT NULL DEFAULT 'medium',       -- easy | medium | hard | expert
  language     TEXT NOT NULL DEFAULT 'en',           -- en | bcs | other
  tags         TEXT NOT NULL DEFAULT '[]',
  data         TEXT NOT NULL DEFAULT '{}',
  size_text    TEXT NOT NULL DEFAULT '',             -- "15 × 15, 78 words"
  ready        INTEGER NOT NULL DEFAULT 0,           -- the last test found nothing to fix
  problems     INTEGER NOT NULL DEFAULT 0,
  tested_at    TEXT,
  photo_id     INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  share_token  TEXT NOT NULL UNIQUE,
  share_on     INTEGER NOT NULL DEFAULT 0,           -- anyone with the link can play
  board        INTEGER NOT NULL DEFAULT 1,           -- players see the best times
  reveal       INTEGER NOT NULL DEFAULT 1,           -- players may ask for a letter / the answer
  timer        INTEGER NOT NULL DEFAULT 1,           -- show a clock while playing
  source       TEXT NOT NULL DEFAULT '',             -- the Pocket app that added it
  source_ref   TEXT NOT NULL DEFAULT '',
  pinned       INTEGER NOT NULL DEFAULT 0,
  archived     INTEGER NOT NULL DEFAULT 0,
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX puzzles_user ON puzzles(user_id, archived, updated_at);
CREATE INDEX puzzles_kind ON puzzles(user_id, kind);

-- A puzzle book: puzzles to print together (answers at the back) or share as one link.
CREATE TABLE books (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  title       TEXT NOT NULL,
  subtitle    TEXT NOT NULL DEFAULT '',
  notes       TEXT NOT NULL DEFAULT '',
  answers     TEXT NOT NULL DEFAULT 'back',          -- back | none
  large       INTEGER NOT NULL DEFAULT 0,            -- large print
  share_token TEXT NOT NULL UNIQUE,
  share_on    INTEGER NOT NULL DEFAULT 0,
  created_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at  TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX books_user ON books(user_id, updated_at);

CREATE TABLE book_items (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  book_id    INTEGER NOT NULL REFERENCES books(id) ON DELETE CASCADE,
  puzzle_id  INTEGER NOT NULL REFERENCES puzzles(id) ON DELETE CASCADE,
  sort       INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX book_items_book ON book_items(user_id, book_id, sort);

-- An escape room, treasure hunt or mystery: stages played one after the other.
CREATE TABLE adventures (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  kind         TEXT NOT NULL DEFAULT 'escape',       -- escape | hunt | mystery | party
  title        TEXT NOT NULL,
  intro        TEXT NOT NULL DEFAULT '',             -- the story players read first
  finale       TEXT NOT NULL DEFAULT '',             -- shown when they finish
  theme        TEXT NOT NULL DEFAULT '',
  date         TEXT NOT NULL DEFAULT '',             -- when it is played
  time         TEXT NOT NULL DEFAULT '',
  place        TEXT NOT NULL DEFAULT '',
  players      TEXT NOT NULL DEFAULT '',             -- who is coming
  minutes      INTEGER NOT NULL DEFAULT 60,          -- time limit (0 = none)
  hint_penalty INTEGER NOT NULL DEFAULT 60,          -- seconds added per hint
  status       TEXT NOT NULL DEFAULT 'draft',        -- draft | ready | done
  notes        TEXT NOT NULL DEFAULT '',             -- host notes
  prep         TEXT NOT NULL DEFAULT '[]',           -- the setting-up checklist
  share_token  TEXT NOT NULL UNIQUE,
  share_on     INTEGER NOT NULL DEFAULT 0,
  board        INTEGER NOT NULL DEFAULT 1,
  calendar_ref TEXT NOT NULL DEFAULT '',
  calendar_url TEXT NOT NULL DEFAULT '',
  photo_id     INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  source       TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX adventures_user ON adventures(user_id, updated_at);

CREATE TABLE stages (
  id           INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  adventure_id INTEGER NOT NULL REFERENCES adventures(id) ON DELETE CASCADE,
  sort         INTEGER NOT NULL DEFAULT 0,
  title        TEXT NOT NULL DEFAULT '',
  story        TEXT NOT NULL DEFAULT '',             -- what players read at this stage
  photo_id     INTEGER REFERENCES photos(id) ON DELETE SET NULL,
  lock         TEXT NOT NULL DEFAULT 'answer',       -- answer | code | word | directions | colors | qr | puzzle | none
  puzzle_id    INTEGER REFERENCES puzzles(id) ON DELETE SET NULL,
  answers      TEXT NOT NULL DEFAULT '[]',           -- accepted answers
  lock_hint    TEXT NOT NULL DEFAULT '',             -- "a 4-digit lock"
  hints        TEXT NOT NULL DEFAULT '[]',           -- [{text, after}] after = minutes at this stage
  hidden       TEXT NOT NULL DEFAULT '',             -- host only: where the clue is hidden
  place_name   TEXT NOT NULL DEFAULT '',
  lat          REAL,
  lng          REAL,
  place_ref    TEXT NOT NULL DEFAULT '',             -- the place in Pocket Places
  unlock       TEXT NOT NULL DEFAULT '',             -- shown when solved ("look under the blue vase")
  props        TEXT NOT NULL DEFAULT '',             -- what the host needs for it
  code         TEXT NOT NULL DEFAULT '',             -- the secret in this stage's QR card
  created_at   TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX stages_adv ON stages(user_id, adventure_id, sort);

-- One go at a puzzle, an adventure or a daily puzzle — yours or a player's
-- through a share link (the row belongs to the puzzle's owner).
CREATE TABLE plays (
  id          INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  what        TEXT NOT NULL,                         -- puzzle | adventure | daily
  ref_id      INTEGER NOT NULL DEFAULT 0,            -- puzzle or adventure id
  kind        TEXT NOT NULL DEFAULT '',              -- the puzzle kind
  day         TEXT NOT NULL DEFAULT '',              -- daily: its date
  via         TEXT NOT NULL DEFAULT 'me',            -- me | link | book
  player      TEXT NOT NULL DEFAULT '',
  play_key    TEXT NOT NULL UNIQUE,                  -- the player's browser keeps it to carry on
  started_at  TEXT NOT NULL,
  finished_at TEXT,
  ms          INTEGER NOT NULL DEFAULT 0,
  hints       INTEGER NOT NULL DEFAULT 0,
  mistakes    INTEGER NOT NULL DEFAULT 0,
  score       INTEGER NOT NULL DEFAULT 0,
  max_score   INTEGER NOT NULL DEFAULT 0,
  stage       INTEGER NOT NULL DEFAULT 0,            -- adventure: stages done
  stage_at    TEXT,                                  -- when the current stage began
  state       TEXT NOT NULL DEFAULT '{}',            -- progress: letters so far, hints shown
  last_at     TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX plays_ref ON plays(user_id, what, ref_id, finished_at);
CREATE INDEX plays_day ON plays(user_id, day);

-- Word lists: words (and clues) to make puzzles from.
CREATE TABLE wordlists (
  id         INTEGER PRIMARY KEY AUTOINCREMENT,
  user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  name       TEXT NOT NULL,
  language   TEXT NOT NULL DEFAULT 'en',
  words      TEXT NOT NULL DEFAULT '[]',             -- [{word, clue}]
  source     TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);
CREATE INDEX wordlists_user ON wordlists(user_id, name);
