package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/urfave/cli/v2"
	"gopkg.in/tucnak/telebot.v2"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

var (
	smtpHost         = "127.0.0.1"
	smtpPort         = "2525"
	smtpAddr         = fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	telegramBotToken = "abc123"
	telegramRoom     = "123456"
)

const smtpLineBreak = "\r\n"
const lineBreak = "\n"

type RecorderService struct {
	messageBody       string
	isMarkdownService bool
}

type mailContent struct {
	contentType string
	content     string
}

type telegramResponse struct {
	Ok          bool
	ErrorCode   int `json:"error_code"`
	Description string
}

func (s *RecorderService) Init(_ map[string]string) error {
	return nil
}

func (s *RecorderService) Send(msg string) error {
	s.messageBody = msg
	return nil
}

func (s *RecorderService) IsMarkdownService() bool {
	return s.isMarkdownService
}

func TestTelegramService(t *testing.T) {
	t.Run("Init", func(t *testing.T) {
		telegramService := &TelegramService{}
		mux := http.NewServeMux()
		_, srv := createStubTelegramBotServer(t, mux)
		flags := generateTestFlags()

		flags[telegramApiUrlFlag] = srv.URL

		t.Run("With valid arguments", func(t *testing.T) {
			err := telegramService.Init(flags)

			if err != nil {
				t.Errorf("Could not start Telegram service: %v", err)
			}

			if telegramService.bot == nil {
				t.Errorf("Telegram bot object not initialized")
			}

			if telegramService.room == nil {
				t.Errorf("Telegram room not initialized")
			}
		})

		t.Run("With invalid arguments", func(t *testing.T) {
			flags[telegramTokenFlag] = "foo"
			err := telegramService.Init(flags)

			if err == nil {
				t.Errorf("Could start Telegram service even though we should not: %v", err)
			}
		})

		t.Run("With missing token", func(t *testing.T) {
			flags = generateTestFlags()
			flags[telegramTokenFlag] = ""
			err := telegramService.Init(flags)
			if err == nil {
				t.Errorf("Could start Telegram service even though we should not: %v", err)
			}

			got := err.Error()
			want := "telegram token not set"
			assertErrorContent(t, got, want)
		})

		t.Run("With missing chat room id", func(t *testing.T) {
			flags = generateTestFlags()
			flags[telegramChatIdFlag] = ""
			err := telegramService.Init(flags)
			if err == nil {
				t.Errorf("Could start Telegram service even though we should not: %v", err)
			}

			got := err.Error()
			want := "telegram chat id not set"
			assertErrorContent(t, got, want)
		})
	})

	t.Run("Send", func(t *testing.T) {
		var tests = []struct {
			name               string
			responseBody       string
			expectedStatusCode int
		}{
			{
				"Correct message",
				`{"ok": true}`,
				http.StatusOK,
			},
			{
				"Invalid information",
				`{"ok": false, "error_code": 400, "description": "invalid information"}`,
				http.StatusBadRequest,
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				var response telegramResponse
				msg := "This _is_ a *strong* email" + lineBreak + lineBreak + "From test"
				sendMessageEndpoint := fmt.Sprintf("/bot%s/sendMessage", telegramBotToken)

				mux := http.NewServeMux()
				mux.Handle(sendMessageEndpoint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					io.WriteString(w, test.responseBody)
				}))

				service, server := createStubTelegramBotServer(t, mux)
				err := service.Send(msg)

				json.Unmarshal([]byte(test.responseBody), &response)

				if response.Ok && err != nil {
					t.Errorf("Error while we weren't supposed to get any: %v", err)
				}

				if !response.Ok && err == nil {
					t.Errorf("We didn't get any error while we were supposed to get one.\n"+
						"Expected error code %d; expected description: %s", response.ErrorCode, response.Description)
				}

				server.Close()
			})
		}
	})
}

func TestAppStart(t *testing.T) {
	t.Run("With valid arguments", func(t *testing.T) {
		args := os.Args[0:1]
		args = append(args, fmt.Sprintf("-smtp-host=%s", smtpHost))
		args = append(args, fmt.Sprintf("-smtp-port=%s", smtpPort))
		args = append(args, fmt.Sprintf("-telegram-token=%s", telegramBotToken))
		args = append(args, fmt.Sprintf("-telegram-chat-id=%s", "1234"))

		err := runStubApp(args)
		if err != nil {
			t.Errorf("Got an error while running the app: %v", err)
		}
	})

	t.Run("With no arguments", func(t *testing.T) {
		args := os.Args[0:1]
		err := runStubApp(args)
		if err == nil {
			t.Errorf("Had no errors while expecting one: %v", err)
		}
	})
}

func assertMessageContent(t *testing.T, testName, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("Test: %s,\nMessage from server: '%s'\n, Message expected: '%s'", testName, got, want)
	}
}

func assertErrorContent(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("error different than %s: %s", want, got)
	}
}

func createStubTelegramBotServer(t *testing.T, mux *http.ServeMux) (*TelegramService, *httptest.Server) {
	t.Helper()
	getMeEndpoint := fmt.Sprintf("/bot%s/getMe", telegramBotToken)
	mux.Handle(getMeEndpoint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true,"result":{"id":123456,"first_name":"a","last_name":"b","user_name":"abc","language_code":"en","is_bot":true,"can_join_groups":true,"can_read_all_group_messages":true,"supports_inline_queries":true}}`)
	}))

	testServer := httptest.NewServer(mux)

	bot, _ := telebot.NewBot(telebot.Settings{
		URL:       testServer.URL,
		Token:     telegramBotToken,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdownV2,
	})

	service := &TelegramService{
		bot:  bot,
		room: &TelegramRoom{id: telegramRoom},
	}

	return service, testServer
}

func runStubApp(args []string) error {
	app := cli.NewApp()
	app.Flags = GenerateCLIFlags()
	app.Action = func(c *cli.Context) error {
		if c.NumFlags() == 0 {
			return errors.New("no flags set")
		}

		smtpConfig, _, _ := generateTestSmtpConfig()
		smtpConfig.port = "0"
		startSmtpServer(smtpConfig, []Service{})

		return nil
	}
	return app.Run(args)
}

func generateTestFlags() map[string]string {
	flags := make(map[string]string)
	flags[smtpHostFlag] = smtpHost
	flags[smtpPortFlag] = smtpPort
	flags[telegramTokenFlag] = telegramBotToken
	flags[telegramChatIdFlag] = "1234"
	return flags
}
