package main

import (
	"context"
	"fmt"
	"github.com/mhale/smtpd"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/smtp"
	"testing"
	"time"
)

var (
	smtpHost         = "127.0.0.1"
	smtpPort         = 2525
	smtpAddr         = fmt.Sprintf("%s:%d", smtpHost, smtpPort)
	telegramBotToken = "abc123"
)

const smtpLineBreak = "\r\n"
const lineBreak = "\n"

type HandlerRecorder struct {
	messageBody string
}

func TestReceiveMessage(t *testing.T) {
	// Init server
	recorder := &HandlerRecorder{}
	srv := &smtpd.Server{
		Addr:     smtpAddr,
		Handler:  recorder.stubHandle,
		Appname:  "TegamiTest",
		Hostname: "",
	}

	go func() {
		srv.ListenAndServe()
	}()

	defer srv.Shutdown(context.Background())
	waitForServer()

	toSubjectFields := "To: test2@test.com" + smtpLineBreak + "Subject: Hello!" + smtpLineBreak + smtpLineBreak

	var tests = []struct {
		name            string
		messageSent     string
		messageExpected string
	}{
		{
			"One-line body",
			toSubjectFields + "This is an email",
			"This is an email",
		},
		{
			"Two-line body",
			toSubjectFields + "This is an email" + smtpLineBreak + "This is another line",
			"This is an email" + lineBreak + "This is another line",
		},
		{
			"Two-line body with newline in-between",
			toSubjectFields + "This is an email" + smtpLineBreak + smtpLineBreak + "This is another line",
			"This is an email" + lineBreak + lineBreak + "This is another line",
		},
		{
			"Two-line body with newline at the end",
			toSubjectFields + "This is an email" + smtpLineBreak + "This is another line" + smtpLineBreak,
			"This is an email" + lineBreak + "This is another line",
		},
		{
			name:            "One-line body with bold attribute",
			messageSent:     toSubjectFields + "This is a <b>strong</b> email",
			messageExpected: "This is a **strong** email",
		},
		{
			name:            "Three-line body with header, italics and bold",
			messageSent:     toSubjectFields + "<h1>Hi</h1>" + smtpLineBreak + "This <i>is</i> a <b>strong</b> email" + smtpLineBreak + "From test",
			messageExpected: "# Hi" + lineBreak + lineBreak + "This _is_ a **strong** email" + lineBreak + "From test",
		},
	}

	for _, test := range tests {
		sendMessage(t, []byte(test.messageSent))
		got := recorder.messageBody
		want := test.messageExpected
		assertMessageContent(t, test.name, got, want)
	}
}

func TestSendTelegram(t *testing.T) {
	msg := "This _is_ a *strong* email" + lineBreak + lineBreak + "From test"
	room := &TelegramRoom{"123456"}
	sendMessageEndpoint := fmt.Sprintf("/bot%s/sendMessage", telegramBotToken)

	t.Run("Correct message", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.Handle(sendMessageEndpoint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok": true}`)
		}))

		bot, server := createStubTelegramBotServer(t, mux)
		defer server.Close()

		err := SendToTelegram(bot, room, msg)

		if err != nil {
			t.Errorf("Error while sending message to Telegram: %v", err)
		}
	})

	t.Run("Invalid information", func(t *testing.T) {
		mux := http.NewServeMux()
		mux.Handle(sendMessageEndpoint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"ok": false, "error_code": 402, "description": "invalid information"}`)
		}))

		bot, server := createStubTelegramBotServer(t, mux)
		defer server.Close()

		err := SendToTelegram(bot, room, msg)

		if err == nil {
			t.Errorf("No error retrieved when we should have a 402: %v", err)
		}
	})
}

func (r *HandlerRecorder) stubHandle(remoteAddr net.Addr, from string, to []string, data []byte) error {
	body, err := ProcessMessage(data)

	if err != nil {
		return err
	}

	r.messageBody = body
	return nil
}

func waitForServer() {
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
	err := smtp.SendMail(fmt.Sprintf("%s:%d", smtpHost, smtpPort), nil, "test@test.com", []string{"test2@test.com"}, msg)

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

func createStubTelegramBotServer(t *testing.T, mux *http.ServeMux) (*TelegramBot, *httptest.Server) {
	t.Helper()
	getMeEndpoint := fmt.Sprintf("/bot%s/getMe", telegramBotToken)
	mux.Handle(getMeEndpoint, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"ok":true,"result":{"id":123456,"first_name":"a","last_name":"b","user_name":"abc","language_code":"en","is_bot":true,"can_join_groups":true,"can_read_all_group_messages":true,"supports_inline_queries":true}}`)
	}))

	testServer := httptest.NewServer(mux)

	bot := &TelegramBot{
		apiUrl: testServer.URL,
		token:  telegramBotToken,
	}

	return bot, testServer
}
