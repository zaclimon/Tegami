package main

import (
	"bytes"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"io"
	"net/mail"
	"strings"
)

func ProcessMessage(data []byte) (string, error) {
	body, err := readMessageBody(data)

	if err != nil {
		return "", err
	}

	markdownBody, err := convertToMarkdown(body)
	trimmedBody := strings.TrimSpace(markdownBody)

	return trimmedBody, err
}

func main() {

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
