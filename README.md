# Tegami ✉️

Tegami is an SMTP relay application for transferring emails to third-party messaging services.
The word comes from the Japanese term for letter. (手紙)

## Supported Messaging Services

- Telegram

## Getting Started

While it's possible to build and run the application manually, it is recommended to use Docker unless you want to build
the app yourself. Below is a quick example for using with Telegram.

`docker run -e TEGAMI_TELEGRAM_TOKEN=<token> -e TEGAMI_TELEGRAM_CHAT_ID=<chat-id> -p 2525:2525 zaclimon/tegami`

You can then route your emails through the address `127.0.0.1:2525`. 

### Building the app yourself

You can still do the following to build the app yourself

1. Clone this repository
2. Import packages with: `go mod download`
3. Build it with: `go build -o ./tegami`
4. Run like any Go based binary. Here's the example from earlier using the binary:
`tegami -telegram-token=<token> -telegram-chat-id=<chat-id>`

Note: You can also use environment variables for running the application. More info on this below.

## Configuration

Below are the flags for the binary and environment variables you can use for configuring the app. They are laid out in
a "flag/environment variable" fashion.

- `smtp-host`/`TEGAMI_SMTP_HOST`: Host address for the application. Default: 127.0.0.1 
- `smtp-port`/`TEGAMI_SMTP_PORT`: Host port for the application: Default: 2525

### Telegram

Note that the usage of Telegram requires a bot token and a chat room id. 
More info on this in the [Telegram documentation](https://core.telegram.org/bots#3-how-do-i-create-a-bot)

- `telegram-api-url`/`TEGAMI_TELEGRAM_API_URL`: Telegram API Server URL. Default: `https://api.telegram.org`
- `telegram-token`/`TEGAMI_TELEGRAM_TOKEN`: Bot token for using Telegram.
- `telegram-chat-id`/`TEGAMI_TELEGRAM_CHAT_ID`: Room ID in which the bot will redirect the messages to.