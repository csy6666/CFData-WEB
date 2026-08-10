//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func launchDesktopWindow(url string, onClosed func()) error {
	browser, err := findDesktopBrowser()
	if err != nil {
		return err
	}
	profileDir, err := os.MkdirTemp("", "cfdata-desktop-")
	if err != nil {
		return fmt.Errorf("create temporary browser profile: %w", err)
	}

	cmd := exec.Command(browser,
		"--app="+url,
		"--new-window",
		"--no-first-run",
		"--no-default-browser-check",
		"--disable-session-crashed-bubble",
		"--user-data-dir="+profileDir,
		"--window-size=1280,900",
	)
	if err := cmd.Start(); err != nil {
		_ = os.RemoveAll(profileDir)
		return fmt.Errorf("start desktop browser: %w", err)
	}

	go func() {
		_ = cmd.Wait()
		_ = os.RemoveAll(profileDir)
		if onClosed != nil {
			onClosed()
		}
	}()
	return nil
}

func findDesktopBrowser() (string, error) {
	if configured := strings.TrimSpace(os.Getenv("CFDATA_DESKTOP_BROWSER")); configured != "" {
		if info, err := os.Stat(configured); err == nil && !info.IsDir() {
			return configured, nil
		}
		return "", fmt.Errorf("CFDATA_DESKTOP_BROWSER is not an executable file: %s", configured)
	}

	for _, name := range []string{"msedge.exe", "chrome.exe"} {
		if path, err := exec.LookPath(name); err == nil {
			return path, nil
		}
	}
	for _, path := range []string{
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Microsoft", "Edge", "Application", "msedge.exe"),
		filepath.Join(os.Getenv("ProgramFiles"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("ProgramFiles(x86)"), "Google", "Chrome", "Application", "chrome.exe"),
	} {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", fmt.Errorf("Microsoft Edge or Google Chrome was not found")
}

func showDesktopError(title, message string) {
	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	messagePtr, _ := syscall.UTF16PtrFromString(message)
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	const mbIconError = 0x10
	_, _, _ = messageBox.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), mbIconError)
}
