package model

import (
	"errors"
	"fmt"
	"os"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

const sessionSecretOptionKey = "SessionSecret"

// InitSessionSecret must run after database migration and before serving requests.
// An explicit environment value takes precedence over the shared database secret.
func InitSessionSecret() error {
	secret := os.Getenv("SESSION_SECRET")
	if secret == "random_string" {
		return errors.New("SESSION_SECRET must be a random string, not the example value 'random_string'")
	}
	if secret == "" {
		var err error
		secret, err = loadOrCreateSessionSecret(DB)
		if err != nil {
			return err
		}
	}

	common.SessionSecret = secret
	common.CryptoSecret = os.Getenv("CRYPTO_SECRET")
	if common.CryptoSecret == "" {
		common.CryptoSecret = secret
	}
	return nil
}

func loadOrCreateSessionSecret(db *gorm.DB) (string, error) {
	// Never include the signing key in SQL logs, including when DEBUG is enabled.
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	var option Option
	err := db.Where(&Option{Key: sessionSecretOptionKey}).Take(&option).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		secret, generateErr := common.GenerateRandomKey(64)
		if generateErr != nil {
			return "", fmt.Errorf("generate session secret: %w", generateErr)
		}
		candidate := Option{Key: sessionSecretOptionKey, Value: secret}
		// Concurrent first starts must keep the winner's key on all three databases.
		if err = db.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; err != nil {
			return "", fmt.Errorf("persist session secret: %w", err)
		}
		err = db.Where(&Option{Key: sessionSecretOptionKey}).Take(&option).Error
	}
	if err != nil {
		return "", fmt.Errorf("load session secret: %w", err)
	}
	if option.Value == "" || option.Value == "random_string" {
		return "", errors.New("stored session secret is invalid; restore it from backup or configure SESSION_SECRET")
	}
	return option.Value, nil
}
