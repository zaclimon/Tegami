package main

import (
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"gopkg.in/tucnak/telebot.v2"
	"log"
	"os"
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
	host string
	port string
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
		ParseMode: telebot.ModeHTML,
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
	smtpHost := c.String(smtpHostFlag)
	smtpPort := c.String(smtpPortFlag)
	smtpAddr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	initServicesCount, services := initServices(RetrieveFlags(c))

	if initServicesCount == 0 {
		log.Fatalln("Couldn't initialize any messaging service, exiting.")
	}

	config := &SmtpConfig{smtpHost, smtpPort}
	srv := CreateSmtpServer(config, services)

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
