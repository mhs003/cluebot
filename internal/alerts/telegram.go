package alerts

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type TelegramBot struct {
	BotToken string
	ChatID   string
	Enabled  bool
	NotifyOn []string
	client   *http.Client
}

type TelegramMessage struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

func NewTelegramBot(botToken, chatID string, enabled bool, notifyOn []string) *TelegramBot {
	return &TelegramBot{
		BotToken: botToken,
		ChatID:   chatID,
		Enabled:  enabled,
		NotifyOn: notifyOn,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (t *TelegramBot) ShouldNotify(incidentType string) bool {
	if !t.Enabled {
		return false
	}

	if len(t.NotifyOn) == 0 {
		return true
	}

	for _, notifyType := range t.NotifyOn {
		if notifyType == incidentType {
			return true
		}
	}

	return false
}

func (t *TelegramBot) SendAlert(incidentType string, message string) error {
	if !t.Enabled {
		return nil
	}

	if !t.ShouldNotify(incidentType) {
		return nil
	}

	emoji := getIncidentEmoji(incidentType)
	text := fmt.Sprintf("%s <b>ClueBot Alert: %s</b>\n\n%s", emoji, incidentType, message)

	msg := TelegramMessage{
		ChatID:    t.ChatID,
		Text:      text,
		ParseMode: "HTML",
	}

	jsonData, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", t.BotToken)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("telegram API returned status %d", resp.StatusCode)
	}

	return nil
}

func getIncidentEmoji(incidentType string) string {
	switch incidentType {
	case "cpu":
		return "🔥"
	case "memory":
		return "💾"
	case "disk":
		return "💿"
	case "process":
		return "⚡"
	case "service":
		return "🔧"
	case "kernel":
		return "🖥️"
	case "restart":
		return "🔄"
	case "port":
		return "🔌"
	default:
		return "⚠️"
	}
}

func FormatIncidentMessage(incidentType string, details map[string]string) string {
	var builder strings.Builder

	builder.WriteString("<b>Server:</b> localhost\n")
	builder.WriteString("<b>Time:</b> " + time.Now().Format("2006-01-02 15:04:05") + "\n\n")

	for key, value := range details {
		builder.WriteString(fmt.Sprintf("<b>%s:</b> %s\n", key, value))
	}

	return builder.String()
}
