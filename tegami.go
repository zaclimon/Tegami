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

// TelegramRoom identifies Telegram chat rooms.
type TelegramRoom struct {
	id string
}

// TelegramService manages Telegram related components.
type TelegramService struct {
	bot  *telebot.Bot
	room *TelegramRoom
}

// SmtpConfig stores the configuration for the SMTP server.
type SmtpConfig struct {
	address  string
	handler  smtpd.Handler
	appName  string
	hostname string
}

// SmtpHandler handles the services in which it must send messages to.
type SmtpHandler struct {
	services []Service
}

// Service is an interface for handling third-party messaging services.
type Service interface {
	// Init ensures the service is initialized based on the flags
	// received by the application and returns an error in case of issues.
	Init(flags map[string]string) error
	// Send transfers the message to the service and returns
	// an error if there was an issue during the transmission.
	Send(msg string) error
	IsMarkdownService() bool
}

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

func (s *TelegramService) IsMarkdownService() bool {
	return false
}

func main() {
	app := cli.NewApp()
	app.Flags = GenerateCLIFlags()
	app.Action = handleCli

	err := app.Run(os.Args)

	if err != nil {
		log.Fatalf("Error while starting the app: %v", err)
	}
}

// ProcessMessage retrieves the data of the message from the SMTP server
// and processes it. Returns the message in its HTML and Markdown form. It also
// returns an error if the message couldn't be processed.
func ProcessMessage(data []byte) (string, string, error) {
	body, err := readMessageBody(data)

	if err != nil {
		return "", "", err
	}

	trimmedBody := strings.TrimSpace(body)
	markdownBody, err := convertToMarkdown(trimmedBody)

	return trimmedBody, markdownBody, err
}

// GenerateCLIFlags returns an array containing all the appropriate flags for the application.
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
			Value:   "https://api.telegram.org",
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

// RetrieveFlags obtains all the values of the flags
func RetrieveFlags(c *cli.Context) map[string]string {
	flagNames := generateFlagNames()
	flags := make(map[string]string)

	for _, flagName := range flagNames {
		flags[flagName] = c.String(flagName)
	}

	return flags
}

// readMessageBody reads the message body from the SMTP server and returns the string of the body.
// It also returns an error if it couldn't properly read the message.
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

// convertToMarkdown converts a string of text to its appropriate Markdown configuration.
func convertToMarkdown(body string) (string, error) {
	converter := md.NewConverter("", true, nil)
	markdownBody, err := converter.ConvertString(body)

	if err != nil {
		return "", err
	}

	return markdownBody, nil
}

// Handle processes SMTP messages to registered services.
func (h *SmtpHandler) Handle(remoteAddr net.Addr, from string, to []string, data []byte) error {
	htmlMsg, markdownMsg, err := ProcessMessage(data)

	if err != nil {
		return err
	}

	for _, service := range h.services {
		var msg string

		if service.IsMarkdownService() {
			msg = markdownMsg
		} else {
			msg = htmlMsg
		}

		if err = service.Send(msg); err != nil {
			fmt.Printf("Could not send message: %s\n", err.Error())
			return err
		}
	}

	return nil
}

// initServices is responsible for initializing all messaging services. It returns the number of
// successfully initialized services as well as a slice of initialized services
func initServices(flags map[string]string) (int, []Service) {
	services := []Service{&TelegramService{}}
	successCount := 0

	for _, service := range services {
		err := service.Init(flags)
		if err != nil {
			fmt.Printf("Error while initializing service: %v\n", err)
		} else {
			successCount++
		}
	}
	return successCount, services
}

// handleCli is the action function when Tegami is started.
func handleCli(c *cli.Context) error {
	smtpAddr := fmt.Sprintf("%s:%s", c.String(smtpHostFlag), c.String(smtpPortFlag))
	initServicesCount, services := initServices(RetrieveFlags(c))

	if initServicesCount == 0 {
		log.Fatalln("Couldn't initialize any messaging service, exiting.")
	}

	handler := &SmtpHandler{services}
	srv := &smtpd.Server{
		Addr:    smtpAddr,
		Handler: handler.Handle,
		Appname: "Tegami",
	}

	fmt.Printf("Starting SMTP Server at address %s\n", smtpAddr)

	if err := srv.ListenAndServe(); err != nil {
		return err
	}

	return nil
}

func generateFlagNames() []string {
	flags := GenerateCLIFlags()
	flagNames := make([]string, len(flags))

	for i, flag := range flags {
		flagNames[i] = flag.Names()[0]
	}

	return flagNames
}
