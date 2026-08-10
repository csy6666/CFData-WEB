package main

import "testing"

func TestNormalizeSHA256(t *testing.T) {
	const digest = "37f253b587a832a0dd3963ef3344cd929f8114560e0ff8ff7fbf92906083c1e7"
	got, err := normalizeSHA256("sha256:" + digest)
	if err != nil || got != digest {
		t.Fatalf("normalizeSHA256 returned %q, %v", got, err)
	}
	if _, err := normalizeSHA256("not-a-digest"); err == nil {
		t.Fatal("normalizeSHA256 accepted an invalid digest")
	}
}

func TestFindDesktopUpdateAsset(t *testing.T) {
	info := latestReleaseInfo{Assets: []releaseAsset{
		{Name: "cfdata-windows-amd64.exe", State: "uploaded", Size: 1, BrowserDownloadURL: "https://github.com/example/plain.exe", Digest: "sha256:37f253b587a832a0dd3963ef3344cd929f8114560e0ff8ff7fbf92906083c1e7"},
		{Name: "cfdata-desktop-windows-amd64.exe", State: "uploaded", Size: 42, BrowserDownloadURL: "https://github.com/example/desktop.exe", Digest: "sha256:37f253b587a832a0dd3963ef3344cd929f8114560e0ff8ff7fbf92906083c1e7"},
	}}
	asset, ok := findDesktopUpdateAsset(info, "amd64")
	if !ok || asset.Name != "cfdata-desktop-windows-amd64.exe" {
		t.Fatalf("findDesktopUpdateAsset chose %+v, %v", asset, ok)
	}
}
