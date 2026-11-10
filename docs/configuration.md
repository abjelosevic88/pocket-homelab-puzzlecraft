# Configuration

Environment variables; every one has a default. Personal settings (language, difficulty, playing, printing, the puzzles of the day, notifications, connections, AI) live in the UI per person — the `OPENAI_*`/`NOTIFY_*` values below are copied once into the first admin's settings.

| Variable | Default | What it does |
|---|---|---|
| `BASE_URL` | – | Where the app is (links in pushes, events and other apps, the OIDC callback) |
| `HOST` / `PORT` | `0.0.0.0` / `8080` | Where it listens |
| `DATA_DIR` | `./data` | The database (pictures are inside it) and the Pocket Core token |
| `DB_PATH` | `$DATA_DIR/pocket-puzzlecraft.db` | The SQLite file |
| `AUTH_MODE` | `local` | `local` (passwords), `proxy` (a header from your reverse proxy), `none` (one user, for trying it out) |
| `SECURE_COOKIES` | `false` | Set `true` behind HTTPS |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` / `ADMIN_NAME` | – | The first account, made on an empty database |
| `OIDC_ISSUER` / `OIDC_CLIENT_ID` / `OIDC_CLIENT_SECRET` | – | Sign in with an OpenID Connect provider (not needed with Pocket Core) |
| `NOTIFY_URL` / `NOTIFY_FORMAT` | – / `apprise` | Pushes: an Apprise URL, an ntfy topic or a JSON webhook |
| `NOTIFY_HOUR` | `8` | Default hour of the daily message (puzzles of the day, what is left to set up for tomorrow's escape room) |
| `OPENAI_API_KEY` / `OPENAI_MODEL` / `OPENAI_BASE_URL` | – | AI without Pocket Core |
| `CORE_URL` / `CORE_REGISTRATION_KEY` / `CORE_APP_URL` | – | Pocket Core |
| `TRUST_PROXY` | `false` | Take the client address from `X-Forwarded-For` (rate limits per address) |
| `METRICS_ENABLED` | `false` | Prometheus `/metrics` |
| `MAX_BODY_BYTES` | `67108864` | Largest import or photo |
| `DEMO_DATA` | `false` | Seed nineteen puzzles of every kind with friends' plays, six weeks of puzzles of the day, two books, word lists and three escape rooms / hunts (one with a team playing) into the owner account (no network calls) |
| `LOG_LEVEL` / `LOG_FORMAT` | `info` / `text` | |

Connections to Pocket Calendar, Contacts, Chat, Email, Photos, Files, Notes, Tasks, Inventory, Places, Map, Learn, Bookshelf, Habits, Quest, Blog, Family, Translate and Alerts are made per person under Settings → Connected apps (Pocket Core, or a URL and token).

