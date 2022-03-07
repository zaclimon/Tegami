package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/emersion/go-message/mail"
	gosmtp "github.com/emersion/go-smtp"
	"github.com/urfave/cli/v2"
	"gopkg.in/tucnak/telebot.v2"
	"io"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"os"
	"strings"
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

func TestSmtpSession(t *testing.T) {
	htmlService := &RecorderService{isMarkdownService: false}
	markdownService := &RecorderService{isMarkdownService: true}
	session := Session{[]Service{htmlService, markdownService}}
	msgContent := "This is a <b>bold</b> message!"

	t.Run("Basic HTML and markdown parsing", func(t *testing.T) {
		msg := createTextMail(t, msgContent)
		err := session.Data(strings.NewReader(msg))

		if err != nil {
			t.Errorf("Something went wrong when reading the message %v", err)
		}

		got := htmlService.messageBody
		want := msgContent
		assertMessageContent(t, t.Name(), got, want)

		got = markdownService.messageBody
		want = "This is a **bold** message!"
		assertMessageContent(t, t.Name(), got, want)
	})

	t.Run("Plain text and html multipart message", func(t *testing.T) {
		msgContentPlain := "This is a Bold message!"

		var tests = []struct {
			name          string
			firstContent  *mailContent
			secondContent *mailContent
		}{
			{
				"Plain text first",
				&mailContent{"text/plain", msgContentPlain},
				&mailContent{"text/html", msgContent},
			},
			{
				"HTML text first",
				&mailContent{"text/html", msgContent},
				&mailContent{"text/plain", msgContentPlain},
			},
		}

		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				writer, buf := createMultipartMailWriter(t)
				addTextMailPart(t, writer, test.firstContent.contentType, test.firstContent.content)
				addTextMailPart(t, writer, test.secondContent.contentType, test.secondContent.content)
				writer.Close()
				err := session.Data(bytes.NewReader(buf.Bytes()))

				if err != nil {
					t.Errorf("Error while processing: %v", err)
				}

				got := htmlService.messageBody
				want := msgContent

				assertMessageContent(t, t.Name(), got, want)
			})
		}
	})
}

func TestServerIntegration(t *testing.T) {
	// Init server
	config, htmlRecorder, markdownRecorder := generateTestSmtpConfig()
	srv := startSmtpServer(config, []Service{htmlRecorder, markdownRecorder})

	defer srv.Close()
	waitForSmtp()

	var tests = []struct {
		name            string
		messageSent     string
		htmlMessage     string
		markdownMessage string
	}{
		{
			"One-line body",
			createTextMail(t, "This is an email"),
			"This is an email",
			"This is an email",
		},
		{
			"Two-line body",
			createTextMail(t, "This is an email"+smtpLineBreak+"This is another line"),
			"This is an email" + smtpLineBreak + "This is another line",
			"This is an email" + lineBreak + "This is another line",
		},
		{
			"Two-line body with newline in-between",
			createTextMail(t, "This is an email"+smtpLineBreak+smtpLineBreak+"This is another line"),
			"This is an email" + smtpLineBreak + smtpLineBreak + "This is another line",
			"This is an email" + lineBreak + lineBreak + "This is another line",
		},
		{
			"Two-line body with newline at the end",
			createTextMail(t, "This is an email"+smtpLineBreak+"This is another line"+smtpLineBreak),
			"This is an email" + smtpLineBreak + "This is another line",
			"This is an email" + lineBreak + "This is another line",
		},
		{
			"One-line body with bold attribute",
			createTextMail(t, "This is a <b>strong</b> email"),
			"This is a <b>strong</b> email",
			"This is a **strong** email",
		},
		{
			"Three-line body with header, italics and bold",
			createTextMail(t, "<h1>Hi</h1>"+smtpLineBreak+"This <i>is</i> a <b>strong</b> email"+smtpLineBreak+"From test"),
			"<h1>Hi</h1>" + smtpLineBreak + "This <i>is</i> a <b>strong</b> email" + smtpLineBreak + "From test",
			"# Hi" + lineBreak + lineBreak + "This _is_ a **strong** email" + lineBreak + "From test",
		},
		{
			"Five-line body using break (br) HTML tags",
			createTextMail(t, "This is an email<br>This is another line<BR>This is a third line<br />This is a fourth line<BR />This is a fifth line"),
			"This is an email" + lineBreak + "This is another line" + lineBreak + "This is a third line" + lineBreak + "This is a fourth line" + lineBreak + "This is a fifth line",
			"This is an email" + lineBreak + "This is another line" + lineBreak + "This is a third line" + lineBreak + "This is a fourth line" + lineBreak + "This is a fifth line",
		},
	}

	for _, test := range tests {
		sendMessage(t, []byte(test.messageSent))
		t.Run(test.name+"-HTML", func(t *testing.T) {
			got := htmlRecorder.messageBody
			want := test.htmlMessage
			assertMessageContent(t, test.name, got, want)
		})

		t.Run(test.name+"-Markdown", func(t *testing.T) {
			got := markdownRecorder.messageBody
			want := test.markdownMessage
			assertMessageContent(t, test.name, got, want)
		})
	}
}

func TestTelegramService(t *testing.T) {
	t.Run("Init", func(t *testing.T) {
		telegramService := &TelegramService{}
		mux := http.NewServeMux()
		_, srv := createStubTelegramBotServer(t, mux)
		flags := generateTestFlags()

		flags[telegramApiUrlFlag] = srv.URL

		t.Run("Init with valid arguments", func(t *testing.T) {
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

		t.Run("Init with invalid arguments", func(t *testing.T) {
			flags[telegramTokenFlag] = "foo"
			err := telegramService.Init(flags)

			if err == nil {
				t.Errorf("Could start Telegram service even though we should not: %v", err)
			}
		})

		t.Run("Init with missing token", func(t *testing.T) {
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

		t.Run("Init with missing chat room id", func(t *testing.T) {
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

func waitForSmtp() {
	// Wait for 5 seconds...
	for i := 0; i < 50; i++ {
		if c, err := smtp.Dial(smtpAddr); err == nil {
			c.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func sendMessage(t *testing.T, msg []byte) {
	t.Helper()
	err := smtp.SendMail(smtpAddr, nil, "test@test.com", []string{"test2@test.com"}, msg)

	if err != nil {
		t.Fatalf("Could not send the messageBody: %v", err)
	}
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

func generateTestSmtpConfig() (*SmtpConfig, *RecorderService, *RecorderService) {
	htmlService := &RecorderService{isMarkdownService: false}
	markdownService := &RecorderService{isMarkdownService: true}

	return &SmtpConfig{
		host: smtpHost,
		port: smtpPort,
	}, htmlService, markdownService
}

func generateTestFlags() map[string]string {
	flags := make(map[string]string)
	flags[smtpHostFlag] = smtpHost
	flags[smtpPortFlag] = smtpPort
	flags[telegramTokenFlag] = telegramBotToken
	flags[telegramChatIdFlag] = "1234"
	return flags
}

func startSmtpServer(config *SmtpConfig, services []Service) *gosmtp.Server {
	srv := CreateSmtpServer(config, services)

	go func() {
		srv.ListenAndServe()
	}()

	return srv
}

func createMultipartMailWriter(t *testing.T) (*mail.InlineWriter, *bytes.Buffer) {
	t.Helper()
	writer, _, buf := createMailWriter(t, false)
	return writer, buf
}

func addTextMailPart(t *testing.T, writer *mail.InlineWriter, contentType string, content string) {
	t.Helper()
	var header mail.InlineHeader
	header.Set("Content-Type", contentType)
	partWriter, err := writer.CreatePart(header)

	if err != nil {
		t.Errorf("Could not create mail part: %v", err)
	}

	io.WriteString(partWriter, content)
	partWriter.Close()
}

func createTextMail(t *testing.T, content string) string {
	t.Helper()
	_, writer, buf := createMailWriter(t, true)

	io.WriteString(writer, content)
	writer.Close()
	return string(buf.Bytes())
}

func createMailWriter(t *testing.T, isSingleInline bool) (*mail.InlineWriter, io.WriteCloser, *bytes.Buffer) {
	t.Helper()
	var inlineWriter *mail.InlineWriter
	var singleInlineWriter io.WriteCloser
	var buffer bytes.Buffer
	var header mail.Header

	if isSingleInline {
		singleInlineWriter, _ = mail.CreateSingleInlineWriter(&buffer, header)
	} else {
		inlineWriter, _ = mail.CreateInlineWriter(&buffer, header)
	}

	return inlineWriter, singleInlineWriter, &buffer
}
