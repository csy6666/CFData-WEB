//go:build !windows

package main

import "fmt"

func launchDesktopWindow(string, func()) error {
	return fmt.Errorf("desktop mode is only available on Windows")
}

func showDesktopError(_, message string) {
	fmt.Println(message)
}
