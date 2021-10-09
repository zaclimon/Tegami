package main

import (
	"bytes"
	md "github.com/JohannesKaufmann/html-to-markdown"
	"gopkg.in/tucnak/telebot.v2"
	"io"
	"net/mail"
	"strings"
	"time"
)

type TelegramBot struct {
	apiUrl string
	token  string
}

type TelegramRoom struct {
	id string
}

func (room *TelegramRoom) Recipient() string {
	return room.id
}

func ProcessMessage(data []byte) (string, error) {
	body, err := readMessageBody(data)

	if err != nil {
		return "", err
	}

	markdownBody, err := convertToMarkdown(body)
	trimmedBody := strings.TrimSpace(markdownBody)

	return trimmedBody, err
}

func SendToTelegram(bot *TelegramBot, room *TelegramRoom, msg string) error {
	b, err := telebot.NewBot(telebot.Settings{
		URL:       bot.apiUrl,
		Token:     bot.token,
		Poller:    &telebot.LongPoller{Timeout: 10 * time.Second},
		ParseMode: telebot.ModeMarkdownV2,
	})

	if err != nil {
		return err
	}

	_, err = b.Send(room, msg)

	if err != nil {
		return err
	}

	return nil
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
