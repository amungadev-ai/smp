package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPersistentAdminPassword(t *testing.T) {
	t.Setenv("DRVT_ADMIN_PASSWORD", "")
	dir := t.TempDir()
	p1, created, err := loadOrCreatePassword(dir, "")
	if err != nil || !created || len(p1) < 20 {
		t.Fatalf("first password: %v", err)
	}
	p2, again, err := loadOrCreatePassword(dir, "")
	if err != nil || again || p2 != p1 {
		t.Fatalf("password changed on restart: %v", err)
	}
	stat, err := os.Stat(filepath.Join(dir, "admin-password"))
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatalf("wrong permissions %v", err)
	}
	os.WriteFile(filepath.Join(dir, "admin-password"), []byte("\n"), 0600)
	if _, _, err = loadOrCreatePassword(dir, ""); err == nil {
		t.Fatal("empty file accepted")
	}
}
