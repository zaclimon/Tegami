package main

import (
	"errors"
	"fmt"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/emersion/go-message"
	"github.com/emersion/go-smtp"
	"io"
	"regexp"
	"strings"
)

var IsNotMultipartError = errors.New("message is not multipart")

type Backend struct {
	services []Service
}

func (bkd *Backend) Login(_ *smtp.ConnectionState, _, _ string) (smtp.Session, error) {
	return nil, nil
}

func (bkd *Backend) AnonymousLogin(_ *smtp.ConnectionState) (smtp.Session, error) {
	return &Session{bkd.services}, nil
}

type Session struct {
	services []Service
}

func (s *Session) AuthPlain(_, _ string) error {
	return nil
}

func (s *Session) Mail(_ string, _ smtp.MailOptions) error {
	return nil
}

func (s *Session) Rcpt(_ string) error {
	return nil
}

func (s *Session) Data(r io.Reader) error {
	htmlMessage, markdownMessage, err := ProcessMessage(r)

	if err != nil {
		return err
	}

	for _, service := range s.services {
		var messageToSend string

		if service.IsMarkdownService() {
			messageToSend = markdownMessage
		} else {
			messageToSend = htmlMessage
		}

		if err = service.Send(messageToSend); err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) Reset() {}

func (s *Session) Logout() error {
	return nil
}

func CreateSmtpServer(config *SmtpConfig, services []Service) *smtp.Server {
	be := &Backend{services}
	srv := smtp.NewServer(be)
	srv.Addr = fmt.Sprintf("%s:%s", config.host, config.port)
	srv.AllowInsecureAuth = true
	return srv
}

// ProcessMessage retrieves the data of the message from the SMTP server
// and processes it. Returns the message in its HTML and Markdown form. It also
// returns an error if the message couldn't be processed.
func ProcessMessage(messageData io.Reader) (string, string, error) {
	body, err := readMessageBody(messageData)

	if err != nil {
		return "", "", err
	}

	// Telegram doesn't accept <br> HTML tags and html-to-markdown adds two newlines instead of one.
	breakRegex := regexp.MustCompile(`(?i)<br>|<br />`)
	body = breakRegex.ReplaceAllString(body, "\n")

	trimmedBody := strings.TrimSpace(body)
	markdownBody, err := convertToMarkdown(trimmedBody)

	return trimmedBody, markdownBody, err
}

// readMessageBody reads the message body from the SMTP server and returns the string of the body.
// It also returns an error if it couldn't properly read the message.
func readMessageBody(data io.Reader) (string, error) {
	msg, err := message.Read(data)

	if err != nil {
		return "", err
	}
	multipartBody, err := readMultipartBody(msg)

	if err != nil && err != IsNotMultipartError {
		return "", err
	} else if err == nil {
		return multipartBody, nil
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

func readMultipartBody(msg *message.Entity) (string, error) {
	var messageBody strings.Builder
	mr := msg.MultipartReader()

	if mr == nil {
		return "", IsNotMultipartError
	}

	for {
		p, err := mr.NextPart()
		if err == io.EOF {
			break
		} else if err != nil {
			return "", err
		}

		contentType, _, _ := p.Header.ContentType()

		if contentType == "text/plain" || contentType == "text/html" {
			bytes, err := io.ReadAll(p.Body)
			if err != nil {
				return "", err
			}

			// Prioritize html messages over plain text ones
			if contentType == "text/html" {
				if messageBody.Len() > 0 {
					messageBody.Reset()
				}
				messageBody.Write(bytes)
				break
			} else {
				messageBody.Write(bytes)
			}
		}
	}
	return messageBody.String(), nil
}
