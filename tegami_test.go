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

	var tests = []struct {
		name            string
		messageSent     string
		messageExpected string
	}{
		{
			"One-line body",
			"To: test2@test.com" + lineBreak + "Subject: Hello!" + lineBreak + lineBreak + "This is an email",
			"This is an email",
		},
		{
			"Two-line body",
			"To: test2@test.com" + lineBreak + "Subject: Hello!" + lineBreak + lineBreak + "This is an email" + lineBreak + "This is another line",
			"This is an email" + lineBreak + "This is another line",
		},
		{
			"Two-line body with newline in-between",
			"To: test2@test.com" + lineBreak + "Subject: Hello!" + lineBreak + lineBreak + "This is an email" + lineBreak + lineBreak + "This is another line",
			"This is an email" + lineBreak + lineBreak + "This is another line",
		},
		{
			"Two-line body with newline at the end",
			"To: test2@test.com" + lineBreak + "Subject: Hello!" + lineBreak + lineBreak + "This is an email" + lineBreak + "This is another line" + lineBreak,
			"This is an email" + lineBreak + "This is another line",
		},
	}

	for _, test := range tests {
		sendMessage(t, []byte(test.messageSent))
		got := recorder.messageBody
		want := test.messageExpected
		assertMessageContent(t, test.name, got, want)
	}
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

func assertMessageContent(t *testing.T, testName, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("Test: %s, Message from server: '%s', Message expected: '%s'", testName, got, want)
	}
}
