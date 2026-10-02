# telegram-alert-server

> **Archived.** This project is no longer maintained. The code works and is
> kept for reference; fork it if you want to build on it.

A minimal Go HTTP server that relays alerts to a Telegram chat through the
Telegram Bot API. Point monitoring scripts, cron jobs or webhooks at it and
the message lands in your chat. It ships as a small distroless container.

It is for people who want a self-hosted alert sink with no dependencies
beyond the Go standard library. It deliberately does not do authentication,
rate limiting, retries, message formatting or multiple chats: run it on a
trusted network or behind a reverse proxy that handles access control.

## Build

```
make build   # static binary ./server
make test    # unit tests
make lint    # gofmt + go vet
make image   # container image via podman (docker build works too)
```

## Run

```
podman run -d \
  -e TELEGRAM_BOT_TOKEN=<your bot token> \
  -e TELEGRAM_CHAT_ID=<your chat id> \
  -p 8080:8080 \
  telegram-alert-server
```

| variable | required | meaning |
|----------|----------|---------|
| `TELEGRAM_BOT_TOKEN` | yes | bot token from [@BotFather](https://t.me/BotFather) |
| `TELEGRAM_CHAT_ID` | yes | numeric chat ID or `@channelname` |
| `PORT` | no | listen port, default `8080` |

The server exits at startup if either required variable is missing.

## Endpoints

| method | path | behaviour |
|--------|------|-----------|
| `GET` | `/health` | returns `ok` |
| `POST` | `/alert` | sends the request body as the message text |
| `POST` | `/webhook` | sends the `text` or `message` field of a JSON body, or the raw body if neither is present |

Bodies are limited to 1 MB. Successful sends return `200 sent`; empty bodies
return `400`, and a failed Telegram call returns `502`.

```
curl http://localhost:8080/health

curl -X POST -d "disk usage above 90%" http://localhost:8080/alert

curl -X POST -H "Content-Type: application/json" \
  -d '{"text":"deploy finished"}' \
  http://localhost:8080/webhook
```

## License

[MIT](LICENSE)
