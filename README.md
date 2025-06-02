# GoSaleBot

[![License: CC BY-NC 4.0](https://img.shields.io/badge/License-CC%20BY--NC%204.0-lightgrey.svg)](LICENSE)

---

![GoSaleBot Banner](https://placehold.co/600x150?text=GoSaleBot)

A modular, production-ready Telegram bot for handling sale posts with moderation, photo support, i18n, admin commands, and full Docker deployment.

---

## Features

- Guided sale post creation (title, description, price, location, photos)
- Moderation workflow (approve/reject in group)
- Multi-language support (English, Czech, Hebrew)
- Admin commands for runtime config and pending review
- SQLite persistent storage
- Easy deployment with Docker & Docker Compose

## Quick Start

```sh
# Clone the repository
git clone https://github.com/<your-username>/GoSaleBot.git
cd GoSaleBot

# Add your Telegram token and group IDs to the .env file
cp .env.example .env
# Edit .env with your values

# Build and run the bot
docker compose up --build
```

## Usage

Once the bot is running, interact with it on Telegram:

```text
/start
```

The bot will guide you through creating a sale post, which will be sent for moderation.

## Documentation

- For full technical details, see [docs/README.md](docs/README.md)
- For design and architecture, see [docs/DesignStructure.adoc](docs/DesignStructure.adoc)

## License

This project is licensed under the Creative Commons Attribution-NonCommercial 4.0 International (CC BY-NC 4.0) License. See the [LICENSE](LICENSE) file for details.

---

## Environment Variables

- `TELEGRAM_TOKEN` – Your Telegram bot token
- `ADMIN_CHAT_ID` – Your Telegram user ID for admin commands
- `MODERATION_CHAT_ID` – (optional) Chat ID for moderation group
- `MODERATION_TOPIC_ID` – (optional) Topic/thread ID for moderation group
- `APPROVED_TOPIC_ID` – (optional) Topic/thread ID for approved group
