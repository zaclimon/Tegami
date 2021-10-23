package main

import (
	"bytes"
	"fmt"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mhale/smtpd"
	"github.com/urfave/cli/v2"
	"gopkg.in/tucnak/telebot.v2"
	"io"
	"log"
	"net/mail"
	"os"
	"strings"
	"time"
)

type TelegramBot struct {
	apiUrl string
	token  string
}

type TelegramRoom struct {
	id string
}

type SmtpConfig struct {
	address  string
	handler  smtpd.Handler
	appName  string
	hostname string
}

func (room *TelegramRoom) Recipient() string {
	return room.id
}

func ProcessMessage(data []byte) (string, error) {
	body, err := readMessageBody(data)

	if err != nil {
		return "", err
	}

	markdownBody, err := convertToMarkdown(body)
	trimmedBody := strings.TrimSpace(markdownBody)

	return trimmedBody, err
}

func SendToTelegram(bot *TelegramBot, room *TelegramRoom, msg string) error {
	b, err := telebot.NewBot(telebot.Settings{
		URL:       bot.apiUrl,
		Token:     bot.token,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdownV2,
	})

	if err != nil {
		return err
	}

	_, err = b.Send(room, msg)

	if err != nil {
		return err
	}

	return nil
}

func main() {
	app := cli.NewApp()
	app.Flags = GenerateCLIFlags()
	app.Action = func(c *cli.Context) error {
		smtpAddr := fmt.Sprintf("%s/%s", c.String("smtp-host"), c.String("smtp-port"))
		smtpConfig := &SmtpConfig{
			address: smtpAddr,
			handler: nil,
			appName: "Tegami",
		}

		StartSMTPServer(smtpConfig)
		return nil
	}
	err := app.Run(os.Args)

	if err != nil {
		log.Fatalf("Error while starting the app: %v", err)
	}
}

func readMessageBody(data []byte) (string, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))

	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(msg.Body)

	if err != nil {
		return "", err
	}

	return string(body), nil
}

func convertToMarkdown(body string) (string, error) {
	converter := md.NewConverter("", true, nil)
	markdownBody, err := converter.ConvertString(body)

	if err != nil {
		return "", err
	}

	return markdownBody, nil
}

func StartSMTPServer(config *SmtpConfig) *smtpd.Server {
	srv := &smtpd.Server{
		Addr:     config.address,
		Handler:  config.handler,
		Appname:  config.appName,
		Hostname: config.hostname,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			log.Fatalf("Could not start the SMTP server: %v", err)
		}
	}()

	return srv
}

func GenerateCLIFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    "smtp-host",
			Value:   "127.0.0.1",
			Usage:   "IP address to bind the host smtp server to",
			EnvVars: []string{"TEGAMI_SMTP_ADDRESS"},
		},
		&cli.StringFlag{
			Name:    "smtp-port",
			Value:   "2525",
			Usage:   "TCP port to bind the smtp server to",
			EnvVars: []string{"TEGAMI_SMTP_PORT"},
		},
		&cli.StringFlag{
			Name:    "telegram-token",
			Usage:   "The token used for the Telegram bot",
			EnvVars: []string{"TEGAMI_TELEGRAM_TOKEN"},
		},
		&cli.StringFlag{
			Name:    "telegram-chat-id",
			Usage:   "The Telegram chat room id in which the email will be transferred to",
			EnvVars: []string{"TEGAMI_TELEGRAM_CHAT_ID"},
		},
	}
}
