package telegrambot

import (
	"encoding/json"
	"os"
	"strconv"
)

var (
	BotToken string
	AdminID  int64
)

// LoadConfig loads bot token and admin id from environment variables or config.json.
// Environment variables BOT_TOKEN and BOT_ADMIN_ID take precedence. If they are
// not set, values are read from a local "config.json" file with fields
// "bot_token" and "admin_id".
func LoadConfig() error {
	if token := os.Getenv("BOT_TOKEN"); token != "" {
		BotToken = token
	}
	if idStr := os.Getenv("BOT_ADMIN_ID"); idStr != "" {
		if id, err := strconv.ParseInt(idStr, 10, 64); err == nil {
			AdminID = id
		}
	}
	if BotToken != "" && AdminID != 0 {
		return nil
	}
	data, err := os.ReadFile("config.json")
	if err != nil {
		return err
	}
	var cfg struct {
		BotToken string `json:"bot_token"`
		AdminID  int64  `json:"admin_id"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return err
	}
	if BotToken == "" {
		BotToken = cfg.BotToken
	}
	if AdminID == 0 {
		AdminID = cfg.AdminID
	}
	return nil
}
