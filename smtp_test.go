package main

import (
	"bytes"
	"github.com/emersion/go-message/mail"
	gosmtp "github.com/emersion/go-smtp"
	"io"
	"net/smtp"
	"strings"
	"testing"
	"time"
)

func TestSmtpSession(t *testing.T) {
	htmlService := &RecorderService{isMarkdownService: false}
	markdownService := &RecorderService{isMarkdownService: true}
	session := TegamiSession{[]Service{htmlService, markdownService}}
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

func generateTestSmtpConfig() (*SmtpConfig, *RecorderService, *RecorderService) {
	htmlService := &RecorderService{isMarkdownService: false}
	markdownService := &RecorderService{isMarkdownService: true}

	return &SmtpConfig{
		host: smtpHost,
		port: smtpPort,
	}, htmlService, markdownService
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
