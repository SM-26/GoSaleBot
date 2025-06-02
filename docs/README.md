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
- `/start` – Begin creating a sale post
- Guided prompts for each sale post field

### Admin Commands
- `/config` – Show all config values
- `/config KEY VALUE` – Set a config value at runtime
- `/pending` – List all pending posts

### Moderation Actions
- **Approve:** Send `/approve` or ✅ in the moderation group
- **Reject:** Reply to a pending post in the moderation group

---

## Directory Structure

- `main.go` – Entrypoint, config/env loading, DB setup, event loop, update handler
- `bot.go` – FSM handler, moderation actions, admin commands, i18n integration
- `db.go` – DB helpers for posts, photos, config
- `fsm.go` – FSM state/session management
- `i18n.go` – Message translations (en, cz, he), i18n.T function
- `main_test.go` – Tests for config, admin, pending, env parsing, DB logic, FSM flow
- `Dockerfile` – Multi-stage build for Go Telegram bot
- `docker-compose.yml` – Service orchestration
- `.env` – Environment variables (not committed to git)
- `docs/` – Design and usage documentation

---

## Further Reading

- [DesignStructure.adoc](./DesignStructure.adoc) – Full design and architecture
- [../README.md](../README.md) – Project overview and quick start

Feel free to add more markdown files for specific topics, such as troubleshooting, advanced configuration, or developer guides.

## To do list:
- [ ] readd the user preview feature
- [ ] remake the whole test file
- [ ] test photo upload