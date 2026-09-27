package data

import (
	"errors"
	"os"
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

// GetSettings returns the stored settings (database only; cheap), with
// GO_CRYPTO_LLM_BASE / GO_CRYPTO_LLM_MODEL overriding the DB values when set.
// The API key itself is never stored on this struct — see GetAPIKey, which
// already falls back to GO_CRYPTO_LLM_API_KEY when no keychain entry exists.
func GetSettings() *models.Settings {
	var s models.Settings
	if err := db.DB.First(&s, 1).Error; err != nil {
		s = defaultSettings()
		db.DB.Create(&s)
	}

	if v := strings.TrimSpace(os.Getenv("GO_CRYPTO_LLM_BASE")); v != "" {
		s.OpenAIBase = v
	}
	if v := strings.TrimSpace(os.Getenv("GO_CRYPTO_LLM_MODEL")); v != "" {
		s.OpenAIModel = v
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
	s.RefreshSecs = ClampRefreshSecs(s.RefreshSecs)
	if k := strings.TrimSpace(s.APIKey); k != "" {
		if err := keyring.Set(keyringService, keyringUser, k); err != nil {
			return errors.New("could not save the API key to the system keychain (" + err.Error() + "). " +
				"On Linux without a keychain, set the " + apiKeyEnv + " environment variable instead")
		}
	}
	s.APIKey = ""
	return db.DB.Save(&s).Error
}

// ClampRefreshSecs keeps the refresh interval within 10s..1h.
func ClampRefreshSecs(secs int) int {
	switch {
	case secs <= 0:
		return 30
	case secs < 10:
		return 10
	case secs > 3600:
		return 3600
	}
	return secs
}

// apiKeyEnv is a fallback for systems without a keychain (e.g. Linux without
// a Secret Service daemon): export GO_CRYPTO_LLM_API_KEY=... before starting.
const apiKeyEnv = "GO_CRYPTO_LLM_API_KEY"

// GetAPIKey reads the key from the OS keychain, falling back to the
// GO_CRYPTO_LLM_API_KEY environment variable. Missing key -> "", nil.
func GetAPIKey() (string, error) {
	k, err := keyring.Get(keyringService, keyringUser)
	if err == nil && k != "" {
		return k, nil
	}
	if env := strings.TrimSpace(os.Getenv(apiKeyEnv)); env != "" {
		return env, nil
	}
	if err == nil || errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return "", err
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
