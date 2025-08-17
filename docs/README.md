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
| `bot/bot.go` | Handles all the message logic from the user. |
| `db/db.go` | DB helpers for posts, photos, config |
| `fsm/fsm.go` | FSM state/session management |
| `i18n/i18n.go` | Message translations (en, cz, he), i18n.T function |
| `main_test.go` | Tests for the main package |
| `Dockerfile` | Multi-stage build for Go Telegram bot |
| `docker-compose.yml` | Service orchestration |
| `.env.example` | Example for environment variables (not committed to git) |
| `docs/` | Design and usage documentation |

---

## Further Reading

- [DesignStructure.adoc](./DesignStructure.adoc) – Full design and architecture
- [../README.md](../README.md) – Project overview and quick start

Feel free to add more markdown files for specific topics, such as troubleshooting, advanced configuration, or developer guides.

## To-do list:

### High-Impact Suggestions
- [ ] **Refactor `HandleMessageWithDB` in `bot/bot.go`**: This function is the core of your bot's logic, but it has become very large and complex.
  - **Suggestion**: Break it down into smaller, single-purpose functions for each state (e.g., `handleIdleState`, `handlePriceState`).
  - **Benefit**: This will make the code easier to read, test, and maintain.
- [ ] **Improve Data Handling in `db/db.go`**: The `SavePostToDB` function currently accepts a `map[string]interface{}` for `postData`.
  - **Suggestion**: Replace the map with a dedicated `Post` struct. This struct would hold all the data for a new post in a type-safe manner.
  - **Benefit**: This will prevent potential runtime errors, make your database logic more robust, and improve code clarity.
- [ ] **Refactor `HandleAdminCommand` in `bot/bot.go`**: Similar to the message handler, the admin command handler is getting long.
  - **Suggestion**: Create a map of command strings to handler functions (e.g., `map[string]func(*sql.DB, ...)`). This is a common and clean pattern for handling sub-commands.
  - **Benefit**: This will make it much easier to add or modify admin commands in the future.