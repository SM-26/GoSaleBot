# GoSaleBot Documentation

Welcome to the GoSaleBot documentation! This folder provides technical and operational details for users, admins, and developers.

---

## Getting Started

1. **Clone the repository:**
   ```sh
   git clone https://github.com/<your-username>/GoSaleBot.git
   cd GoSaleBot
   ```
2. **Configure environment variables:**
   - Copy `.env.example` to `.env` (or create `.env`) and fill in:
     - `TELEGRAM_TOKEN` – Your Telegram bot token
     - `MODERATION_GROUP_ID` – Telegram group ID for moderation
     - `APPROVED_GROUP_ID` – Telegram group ID for approved posts
     - `MODERATION_TOPIC_ID` – (optional) Topic/thread ID for moderation group
     - `APPROVED_TOPIC_ID` – (optional) Topic/thread ID for approved group
     - `LANG` – Default language (en/cz/he)
     - `TIMEOUT_MINUTES` – Post expiration timeout (default: 1440)
     - `ADMINS` - Comma-separated list of user IDs that are admins.
     - `VALIDATE_PRICE` – Enable/disable server-side price validation (true/false). Default: true
     - `MIN_PHOTOS` – Minimum number of photos required to submit a post. Default: 1
3. **Build and run with Docker Compose:**
   ```sh
   docker compose up --build
   ```

---

## Features

- Guided sale post creation (title, description, price, location, photos)
- Moderation workflow (approve via ✅, reject via reply)
- Multi-language support (English, Czech, Hebrew)
- Admin commands for runtime config and pending review
- SQLite persistent storage
- Inline keyboard for photo stage
- Configurable via environment and runtime admin commands
- Handles users without a username by using their first and last name.

---

## Deployment

- **Docker:**
  - Uses a multi-stage Dockerfile for minimal image size.
  - Persistent data stored in `./data` (mounted as a volume).
- **Docker Compose:**
  - Orchestrates the bot and manages environment variables.
- **Environment Variables:**
  - See `.env` for all required and optional variables.
- **Production:**
  - Deploy on any cloud or VPS with Docker support.

---

## API Reference / Bot Commands

### User Commands

| Command | Description |
| --- | --- |
| `/start` | Begin creating a sale post |
| `/help` | Shows the help message with all the available commands. |

### Admin Commands

| Command | Description |
| --- | --- |
| `/config` | Show all config values |
| `/config KEY VALUE` | Set a config value at runtime |
| `/pending` | List all pending posts |
| `/showdb` | Dumps the content of the database. |
| `/cleardb` | Clears the database from all the posts and images. |



### Moderation Actions
- **Approve:** Send `/approve` or ✅ in the moderation group
- **Reject:** Reply to a pending post in the moderation group

---

## Directory Structure

| Path | Description |
| --- | --- |
| `main.go` | Entrypoint, config/env loading, DB setup, event loop, update handler |
| `bot/` | Bot package (see files below) — message handlers, validators, admin commands, and tests |
| `db/` | DB helpers and models (e.g. `db/db.go`) |
| `data/` | Persistent data folder used by Docker volumes (SQLite DB files live here in production) |
| `fsm/` | FSM state/session management (e.g. `fsm/fsm.go`) |
| `i18n/` | Message translations (en, cz, he), `i18n.T` function |
| `docs/` | Design and usage documentation |
| `Dockerfile` | Multi-stage build for Go Telegram bot |
| `docker-compose.yml` | Service orchestration |
| `.env.example` | Example for environment variables (copy to `.env`) |
| `main_test.go` | Tests for the main package |

### `bot/` package (selected files)

| File | Purpose |
| --- | --- |
| `bot.go` | Core dispatcher, session lifecycle, callback handling |
| `handlers.go` | Pure per-state handlers returning `HandlerResult` |
| `handler_result.go` | `HandlerResult` type and `ExecuteHandlerResult` executor that applies side-effects |
| `validators.go` | Pluggable validators (price, photos) and default implementations |
| `admin_commands.go` | Admin command handlers map and helpers (e.g. `/showdb`, `/pending`) |
| `*.go` tests | Unit tests for bot logic (e.g. `bot_test.go`, `admin_commands_test.go`) |

This structure reflects the current repository layout. If you want the table to include every source file or additional explanations (DB schema, config keys), I can expand it further.

---

## Further Reading

- [DesignStructure.adoc](./DesignStructure.adoc) – Full design and architecture
- [../README.md](../README.md) – Project overview and quick start

Feel free to add more markdown files for specific topics, such as troubleshooting, advanced configuration, or developer guides.

## To-do list:
- [ ] add an option for the end user to view all of his listing, check for each post it's status (rejected, approved, pending).
- [ ] add an option for the user to also mark a listing as 'sold', a post marked as such should also update it's status in the 'approved group'.
- [X] add validations in the post creation process, all of the validations should be controlled by the config table.
  - [X] price validation - price must be a numerical value.
  - [X] picture validation - a post must have at least one photo. 
- [ ] add an option for admins to post a broadcast message to all of the users, e.g. "bot is going down for maintenance".
  - [ ] add an option in the config table to automatically send a message when the bot is starting up and shutting down.

Additional suggested next steps (from recent refactor):
- [ ] add `/myposts` pagination or inline-button UI for better UX (currently supports text commands). (Recommended)
- [ ] implement a way to mark approved-group posts as "sold" (update or delete the message in the approved group). (Medium)
- [ ] add admin `/broadcast` command (consider rate-limiting / job queue for many users). (Medium)
- [ ] add more i18n keys for any new admin/user flows and date formatting per-locale. (Low)
- [ ] run `golangci-lint` and fix style/complexity warnings (dev tooling). (Low)

### High-Impact Suggestions
- [X] **Refactor `HandleMessageWithDB` in `bot/bot.go`**: This function is the core of your bot's logic, but it has become very large and complex.
  - **Suggestion**: Break it down into smaller, single-purpose functions for each state (e.g., `handleIdleState`, `handlePriceState`).
  - **Benefit**: This will make the code easier to read, test, and maintain.
- [X] **Improve Data Handling in `db/db.go`**: The `SavePostToDB` function currently accepts a `map[string]interface{}` for `postData`.
  - **Suggestion**: Replace the map with a dedicated `Post` struct. This struct would hold all the data for a new post in a type-safe manner.
  - **Benefit**: This will prevent potential runtime errors, make your database logic more robust, and improve code clarity.
- [X] **Refactor `HandleAdminCommand` in `bot/bot.go`**: Similar to the message handler, the admin command handler is getting long.
  - **Suggestion**: Create a map of command strings to handler functions (e.g., `map[string]func(*sql.DB, ...)`). This is a common and clean pattern for handling sub-commands.
  - **Benefit**: This will make it much easier to add or modify admin commands in the future.