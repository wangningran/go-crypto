package data

import (
	"errors"
	"strings"

	"go-crypto/backend/db"
	"go-crypto/backend/logger"
	"go-crypto/backend/models"

	"github.com/zalando/go-keyring"
)

const (
	keyringService = "go-crypto"
	keyringUser    = "llm-api-key"
)

func defaultSettings() models.Settings {
	return models.Settings{
		ID:          1,
		OpenAIBase:  "https://api.openai.com/v1",
		OpenAIModel: "gpt-4o-mini",
		Currency:    "usd",
		RefreshSecs: 30,
	}
}

// GetSettings returns the stored settings (database only; cheap).
func GetSettings() *models.Settings {
	var s models.Settings
	if err := db.DB.First(&s, 1).Error; err != nil {
		s = defaultSettings()
		db.DB.Create(&s)
	}
	return &s
}

// GetSettingsView is GetSettings plus whether an API key is saved and its
// last 4 characters, for the Settings page. The key itself is never returned.
// It reads the OS keychain, so it is only used by the UI, not on hot paths.
func GetSettingsView() *models.Settings {
	s := GetSettings()
	if key, err := GetAPIKey(); err == nil && key != "" {
		s.HasAPIKey = true
		if len(key) > 4 {
			s.APIKeyHint = "…" + key[len(key)-4:]
		}
	}
	return s
}

// SaveSettings persists settings. A non-empty APIKey replaces the stored key.
func SaveSettings(s models.Settings) error {
	s.ID = 1
	s.Currency = strings.ToLower(strings.TrimSpace(s.Currency))
	if s.Currency == "" {
		s.Currency = "usd"
	}
	if k := strings.TrimSpace(s.APIKey); k != "" {
		if err := keyring.Set(keyringService, keyringUser, k); err != nil {
			return errors.New("could not save the API key to the system keychain: " + err.Error())
		}
	}
	s.APIKey = ""
	return db.DB.Save(&s).Error
}

// GetAPIKey reads the key from the OS keychain. Missing key -> "", nil.
func GetAPIKey() (string, error) {
	k, err := keyring.Get(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return k, err
}

// ClearAPIKey removes the key from the keychain.
func ClearAPIKey() error {
	err := keyring.Delete(keyringService, keyringUser)
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}

// MigrateLegacyAPIKey moves a key saved by older versions (plain text in the
// settings table) into the keychain and blanks the old column.
func MigrateLegacyAPIKey() {
	m := db.DB.Migrator()
	if !m.HasColumn(&models.Settings{}, "open_ai_key") {
		return
	}
	var legacy string
	row := db.DB.Raw("SELECT open_ai_key FROM settings WHERE id = 1").Row()
	if err := row.Scan(&legacy); err != nil || strings.TrimSpace(legacy) == "" {
		return
	}
	if existing, _ := GetAPIKey(); existing == "" {
		if err := keyring.Set(keyringService, keyringUser, strings.TrimSpace(legacy)); err != nil {
			logger.Log.Warnf("legacy API key not migrated (keychain unavailable): %v", err)
			return
		}
	}
	db.DB.Exec("UPDATE settings SET open_ai_key = '' WHERE id = 1")
	logger.Log.Info("moved legacy API key from SQLite to the system keychain")
}
