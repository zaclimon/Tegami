package main

import (
	"context"
	"fmt"
	"github.com/mhale/smtpd"
	"net"
	"net/smtp"
	"testing"
	"time"
)

var (
	smtpHost = "127.0.0.1"
	smtpPort = 2525
	smtpAddr = fmt.Sprintf("%s:%d", smtpHost, smtpPort)
)

const lineBreak = "\r\n"

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

	t.Run("One-line body", func(t *testing.T) {
		msg := []byte("To: test2@test.com" + lineBreak +
			"Subject: Hello!" + lineBreak +
			lineBreak +
			"This is an email")

		sendMessage(t, msg)

		got := recorder.messageBody
		want := "This is an email"
		assertMessageContent(t, got, want)
	})

	t.Run("Two-line body", func(t *testing.T) {
		msg := []byte("To: test2@test.com" + lineBreak +
			"Subject: Hello!" + lineBreak +
			lineBreak +
			"This is an email" +
			lineBreak +
			"This is another line")

		sendMessage(t, msg)

		got := recorder.messageBody
		want := "This is an email" + lineBreak + "This is another line"
		assertMessageContent(t, got, want)
	})

	t.Run("Two-line body with newline between", func(t *testing.T) {
		msg := []byte("To: test2@test.com" + lineBreak +
			"Subject: Hello!" + lineBreak +
			lineBreak +
			"This is an email" +
			lineBreak + lineBreak +
			"This is another line")

		sendMessage(t, msg)

		got := recorder.messageBody
		want := "This is an email" + lineBreak + lineBreak + "This is another line"
		assertMessageContent(t, got, want)
	})

	t.Run("Two-line body with newline at the end", func(t *testing.T) {
		msg := []byte("To: test2@test.com" + lineBreak +
			"Subject: Hello!" + lineBreak +
			lineBreak +
			"This is an email" +
			lineBreak +
			"This is another line" + lineBreak)

		sendMessage(t, msg)

		got := recorder.messageBody
		want := "This is an email" + lineBreak + "This is another line"
		assertMessageContent(t, got, want)
	})
}

func (r *HandlerRecorder) stubHandle(remoteAddr net.Addr, from string, to []string, data []byte) error {
	body, err := ReadMessageBody(data)

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

func assertMessageContent(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("Message from server: '%s', Message expected: '%s'", got, want)
	}
}
