package data

import (
	"go-crypto/backend/db"
	"go-crypto/backend/models"
)

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
	return &s
}

func SaveSettings(s models.Settings) error {
	s.ID = 1
	return db.DB.Save(&s).Error
}
