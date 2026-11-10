# Backup

Everything lives in `DATA_DIR`: the SQLite file `pocket-puzzlecraft.db` (WAL mode; pictures are inside it). Take a consistent copy with `pocket-puzzlecraft backup` (writes `DATA_DIR/backups/pocket-puzzlecraft-<time>.db` with `VACUUM INTO`) or Settings → Backup & data → Download database (admin). Back up that copy, not the live file. `DATA_DIR/core.token*` is the Pocket Core registration.

Restore: stop the app, replace `pocket-puzzlecraft.db` (remove the `-wal`/`-shm` files), start.

Every person can also export their own puzzles, books, escape rooms and word lists (with their pictures) as JSON (`/api/v1/export.json`, back in with Settings → Backup & data — twice is the same as once) and a table of their puzzles as CSV (`/api/v1/export.csv`). Crosswords download as `.ipuz` and `.puz`.
