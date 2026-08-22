package repository

import (
	"fmt"
	"testing"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
)

func TestNewReadOnlyDBDoesNotRunMigrations(t *testing.T) {
	database, err := NewReadOnlyDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatal(err)
	}
	if database.Migrator().HasTable(&model.User{}) {
		t.Fatal("read-only connection must not create application tables")
	}
}
