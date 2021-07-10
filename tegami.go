package main

import (
	"bytes"
	"io"
	"net/mail"
	"strings"
)

func ReadMessageBody(data []byte) (string, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(data))

	if err != nil {
		return "", err
	}

	body, err := io.ReadAll(msg.Body)

	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(body)), nil
}

func main() {

}
