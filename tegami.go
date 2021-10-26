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

type TelegramRoom struct {
	id string
}

type TelegramService struct {
	bot  *telebot.Bot
	room *TelegramRoom
}

type SmtpConfig struct {
	address  string
	handler  smtpd.Handler
	appName  string
	hostname string
}

type Service interface {
	Init(flags map[string]string) error
	Send(msg string) error
}

func (r *TelegramRoom) Recipient() string {
	return r.id
}

func (s *TelegramService) Init(flags map[string]string) error {
	apiUrl := flags["telegram-api-url"]
	token := flags["telegram-token"]

	bot, err := telebot.NewBot(telebot.Settings{
		URL:       apiUrl,
		Token:     token,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdownV2,
	})

	if err != nil {
		return err
	}

	s.bot = bot

	return nil
}

func (s *TelegramService) Send(msg string) error {
	_, err := s.bot.Send(s.room, msg)
	if err != nil {
		return err
	}
	return nil
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
			Name:    "telegram-api-url",
			Usage:   "The API url used for communicating with Telegram (Optional)",
			EnvVars: []string{"TEGAMI_TELEGRAM_API_URL"},
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

func RetrieveFlags(c *cli.Context) map[string]string {
	flagNames := c.FlagNames()
	flags := make(map[string]string)

	for _, name := range flagNames {
		flags[name] = c.String(name)
	}

	return flags
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

		flags := RetrieveFlags(c)
		services := []Service{&TelegramService{}}

		for _, service := range services {
			err := service.Init(flags)
			if err != nil {
				log.Fatalf("Error while initializing service: %v", err)
			}
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
