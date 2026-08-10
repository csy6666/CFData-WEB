//go:build windows

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestVerifyUpdateFileSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "update.exe")
	content := []byte("new desktop binary")
	if err := os.WriteFile(path, content, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(content)
	if err := verifyUpdateFileSHA256(path, hex.EncodeToString(digest[:])); err != nil {
		t.Fatal(err)
	}
}

func TestReplaceUpdateTarget(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "source.exe")
	target := filepath.Join(dir, "target.exe")
	if err := os.WriteFile(source, []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := replaceUpdateTarget(source, target); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "new" {
		t.Fatalf("target contains %q, want new", content)
	}
}
