package data

import (
	"go-crypto/backend/db"
	"go-crypto/backend/models"
	"os"
	"strings"
)

// GetSettings loads settings from DB, then overrides with env vars if set.
// GO_CRYPTO_LLM_API_KEY  — overrides OpenAIKey
// GO_CRYPTO_LLM_BASE     — overrides OpenAIBase
// GO_CRYPTO_LLM_MODEL    — overrides OpenAIModel
func GetSettings() *models.Settings {
	var s models.Settings
	result := db.DB.First(&s, 1)
	if result.Error != nil {
		s = models.Settings{
			ID:          1,
			OpenAIBase:  "https://api.openai.com/v1",
			OpenAIModel: "gpt-4o-mini",
			Currency:    "usd",
			RefreshSecs: 30,
		}
		db.DB.Create(&s)
	}

	if v := strings.TrimSpace(os.Getenv("GO_CRYPTO_LLM_API_KEY")); v != "" {
		s.OpenAIKey = v
	}
	if v := strings.TrimSpace(os.Getenv("GO_CRYPTO_LLM_BASE")); v != "" {
		s.OpenAIBase = v
	}
	if v := strings.TrimSpace(os.Getenv("GO_CRYPTO_LLM_MODEL")); v != "" {
		s.OpenAIModel = v
	}

	return &s
}

func SaveSettings(s models.Settings) error {
	s.ID = 1
	return db.DB.Save(&s).Error
}
