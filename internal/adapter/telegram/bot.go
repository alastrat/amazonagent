package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"
)

type Bot struct {
	token      string
	apiBase    string
	httpClient *http.Client
	commands   *CommandHandler
	stopCh     chan struct{}
}

func NewBot(token string, commands *CommandHandler) *Bot {
	return &Bot{
		token:      token,
		apiBase:    "https://api.telegram.org/bot" + token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
		commands:   commands,
		stopCh:     make(chan struct{}),
	}
}

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message,omitempty"`
}

type Message struct {
	MessageID int    `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Text      string `json:"text"`
	From      *User  `json:"from,omitempty"`
}

type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	Username  string `json:"username"`
}

func (b *Bot) StartPolling(ctx context.Context) {
	slog.Info("telegram: bot polling started")
	offset := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-b.stopCh:
			return
		default:
		}

		updates, err := b.getUpdates(offset, 30)
		if err != nil {
			slog.Warn("telegram: poll error", "error", err)
			time.Sleep(5 * time.Second)
			continue
		}

		for _, update := range updates {
			offset = update.UpdateID + 1
			if update.Message != nil && update.Message.Text != "" {
				go b.handleMessage(ctx, update.Message)
			}
		}
	}
}

func (b *Bot) Stop() { close(b.stopCh) }

func (b *Bot) handleMessage(ctx context.Context, msg *Message) {
	text := strings.TrimSpace(msg.Text)
	chatID := msg.Chat.ID
	slog.Info("telegram: message received", "chat_id", chatID, "text", text)
	response := b.commands.Handle(ctx, chatID, text)
	if response != "" {
		if err := b.SendMessage(chatID, response); err != nil {
			slog.Warn("telegram: send response failed", "error", err)
		}
	}
}

func (b *Bot) getUpdates(offset, timeout int) ([]Update, error) {
	url := fmt.Sprintf("%s/getUpdates?offset=%d&timeout=%d", b.apiBase, offset, timeout)
	resp, err := b.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var result struct {
		OK     bool     `json:"ok"`
		Result []Update `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode updates: %w", err)
	}
	return result.Result, nil
}

func (b *Bot) SendMessage(chatID int64, text string) error {
	body, _ := json.Marshal(map[string]any{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "HTML",
	})
	resp, err := b.httpClient.Post(b.apiBase+"/sendMessage", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("send message: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("send failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	return nil
}
