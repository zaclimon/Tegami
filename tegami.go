package main

import (
	"bytes"
	"errors"
	"fmt"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/mhale/smtpd"
	"github.com/urfave/cli/v2"
	"gopkg.in/tucnak/telebot.v2"
	"io"
	"log"
	"net"
	"net/mail"
	"os"
	"strings"
	"time"
)

const (
	smtpHostFlag       = "smtp-host"
	smtpPortFlag       = "smtp-port"
	telegramApiUrlFlag = "telegram-api-url"
	telegramTokenFlag  = "telegram-token"
	telegramChatIdFlag = "telegram-chat-id"
	smtpHostEnv        = "TEGAMI_SMTP_HOST"
	smtpPortEnv        = "TEGAMI_SMTP_PORT"
	telegramApiUrlEnv  = "TEGAMI_TELEGRAM_API_URL"
	telegramTokenEnv   = "TEGAMI_TELEGRAM_TOKEN"
	telegramChatIdEnv  = "TEGAMI_TELEGRAM_CHAT_ID"
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

var services []Service

func (r *TelegramRoom) Recipient() string {
	return r.id
}

func (s *TelegramService) Init(flags map[string]string) error {
	apiUrl := flags[telegramApiUrlFlag]
	token := flags[telegramTokenFlag]
	chatId := flags[telegramChatIdFlag]

	if len(token) == 0 {
		return errors.New("telegram token not set")
	}

	if len(chatId) == 0 {
		return errors.New("telegram chat id not set")
	}

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
	s.room = &TelegramRoom{id: chatId}

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

func GenerateCLIFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:    smtpHostFlag,
			Value:   "127.0.0.1",
			Usage:   "IP address to bind the host smtp server to",
			EnvVars: []string{smtpHostEnv},
		},
		&cli.StringFlag{
			Name:    smtpPortFlag,
			Value:   "2525",
			Usage:   "TCP port to bind the smtp server to",
			EnvVars: []string{smtpPortEnv},
		},
		&cli.StringFlag{
			Name:    telegramApiUrlFlag,
			Usage:   "The API url used for communicating with Telegram (Optional)",
			EnvVars: []string{telegramApiUrlEnv},
		},
		&cli.StringFlag{
			Name:    telegramTokenFlag,
			Usage:   "The token used for the Telegram bot",
			EnvVars: []string{telegramTokenEnv},
		},
		&cli.StringFlag{
			Name:    telegramChatIdFlag,
			Usage:   "The Telegram chat room id in which the email will be transferred to",
			EnvVars: []string{telegramChatIdEnv},
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
	app.Action = initApp

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

func smtpHandle(remoteAddr net.Addr, from string, to []string, data []byte) error {
	msg, err := ProcessMessage(data)

	if err != nil {
		return err
	}

	for _, service := range services {
		if err = service.Send(msg); err != nil {
			fmt.Printf("Could not send message: %s\n", err.Error())
			return err
		}
	}

	return nil
}

func initServices(flags map[string]string) error {
	services = []Service{&TelegramService{}}

	for _, service := range services {
		err := service.Init(flags)
		if err != nil {
			return err
		}
	}
	return nil
}

func initApp(c *cli.Context) error {
	smtpAddr := fmt.Sprintf("%s:%s", c.String(smtpHostFlag), c.String(smtpPortFlag))
	srv := &smtpd.Server{
		Addr:    smtpAddr,
		Handler: smtpHandle,
		Appname: "Tegami",
	}

	if err := initServices(RetrieveFlags(c)); err != nil {
		log.Fatalf("Error while initializing service: %v", err)
	}

	fmt.Printf("Starting SMTP Server at address %s\n", smtpAddr)

	if err := srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}
