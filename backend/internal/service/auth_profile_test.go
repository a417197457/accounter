package service

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"accounter/backend/internal/config"
	"accounter/backend/internal/model"
	"accounter/backend/internal/repository"
)

func TestUpdateWechatProfile(t *testing.T) {
	db, err := repository.NewDB(&config.Config{
		DB:     config.DBConfig{Driver: "sqlite"},
		SQLite: config.SQLiteConfig{Path: fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())},
	})
	if err != nil {
		t.Fatalf("db: %v", err)
	}
	user := model.User{Username: "wx_profile_user", PasswordHash: "x", Nickname: "微信用户"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatal(err)
	}
	svc := &AuthService{db: db}

	updated, err := svc.UpdateNickname(user.ID, "  账小鸭用户  ")
	if err != nil {
		t.Fatalf("update nickname: %v", err)
	}
	if updated.Nickname != "账小鸭用户" {
		t.Fatalf("nickname not trimmed: %q", updated.Nickname)
	}
	if _, err := svc.UpdateNickname(user.ID, strings.Repeat("昵", 33)); err == nil {
		t.Fatal("nickname longer than 32 characters should fail")
	}

	var avatar bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 66, G: 195, B: 156, A: 255})
	if err := png.Encode(&avatar, img); err != nil {
		t.Fatal(err)
	}
	uploadDir := t.TempDir()
	updated, err = svc.UpdateAvatar(user.ID, uploadDir, avatar.Bytes(), ".png")
	if err != nil {
		t.Fatalf("update avatar: %v", err)
	}
	if !strings.HasPrefix(updated.AvatarURL, fmt.Sprintf("/api/uploads/avatars/%d.png?v=", user.ID)) {
		t.Fatalf("unexpected avatar URL: %s", updated.AvatarURL)
	}
	storedPath := filepath.Join(uploadDir, "avatars", fmt.Sprintf("%d.png", user.ID))
	if info, err := os.Stat(storedPath); err != nil || info.Size() == 0 {
		t.Fatalf("avatar file not stored: info=%v err=%v", info, err)
	}
}
