package handlers

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestKVGetSet(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&ServerKV{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	if err := SetKV(db, "k1", "v1"); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, ok, err := GetKV(db, "k1")
	if err != nil || !ok || v != "v1" {
		t.Fatalf("get mismatch: ok=%v v=%q err=%v", ok, v, err)
	}
}
