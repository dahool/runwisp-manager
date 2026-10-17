# Telegram notifications

Configure the bot token through the RunWisp environment, not as a literal in
the Git-managed TOML. The production compose example reads `TELEGRAM_BOT_TOKEN`
from `runwisp.env`, and mounts that file read-only into the manager for
validation.

```toml
[notifiers.ops-telegram]
type = "telegram"
bot_token = "${TELEGRAM_BOT_TOKEN}"
chat_id = "-1001234567890"
```

`chat_id` is a string, including for negative group/channel IDs. Restrict the
environment file to the RunWisp and manager services, rotate the token if it
is exposed, and avoid putting literal tokens in config commits. See the
[schema notes](../A1-appendix-runwisp-schema-facts.md).
