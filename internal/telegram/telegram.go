package telegram

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
)

func SendBugReport(message string) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		return errors.New("TELEGRAM_BOT_TOKEN not set in .env")
	}

	chatID := "886090434"

	formattedMsg := fmt.Sprintf("🐛 *New Bug Report*\n\n*From:* Harshal Patel\n*Message:*\n%s", message)

	payload := map[string]string{
		"chat_id":    chatID,
		"text":       formattedMsg,
		"parse_mode": "Markdown",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned status: %d", resp.StatusCode)
	}

	return nil
}
