package handlers

import (
	"errors"
	"strings"

	"gorm.io/gorm"
)

func GetKV(db *gorm.DB, key string) (string, bool, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return "", false, nil
	}
	var kv ServerKV
	err := db.First(&kv, "key = ?", k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", false, nil
		}
		return "", false, err
	}
	return kv.Value, true, nil
}

func SetKV(db *gorm.DB, key string, value string) error {
	k := strings.TrimSpace(key)
	if k == "" {
		return nil
	}
	var kv ServerKV
	err := db.First(&kv, "key = ?", k).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			kv = ServerKV{Key: k, Value: value}
			return db.Create(&kv).Error
		}
		return err
	}
	kv.Value = value
	return db.Save(&kv).Error
}
