package main

import (
	"path/filepath"
	"testing"
)

func TestDesktopRuntimeDirectory(t *testing.T) {
	path := filepath.Join("portable", "CFData-Desktop.exe")
	directory, err := desktopRuntimeDirectory(path)
	if err != nil {
		t.Fatal(err)
	}
	if directory != "portable" {
		t.Fatalf("desktopRuntimeDirectory(%q) = %q, want portable", path, directory)
	}
	if _, err := desktopRuntimeDirectory(""); err == nil {
		t.Fatal("desktopRuntimeDirectory accepted an empty path")
	}
}

func TestResolveListenAddress(t *testing.T) {
	tests := []struct {
		host        string
		port        int
		wantAddr    string
		wantDisplay string
	}{
		{host: "", port: 13335, wantAddr: ":13335", wantDisplay: "localhost"},
		{host: " 127.0.0.1 ", port: 0, wantAddr: "127.0.0.1:0", wantDisplay: "127.0.0.1"},
	}
	for _, test := range tests {
		addr, display := resolveListenAddress(test.host, test.port)
		if addr != test.wantAddr || display != test.wantDisplay {
			t.Fatalf("resolveListenAddress(%q, %d) = (%q, %q), want (%q, %q)", test.host, test.port, addr, display, test.wantAddr, test.wantDisplay)
		}
	}
}
