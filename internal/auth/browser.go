package auth

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openURL is the seam tests replace to avoid launching a real browser.
var openURL = defaultOpen

// Open launches the URL in the system default browser via the injectable seam.
// On failure it prints the URL so headless/SSH environments still work.
func Open(url string) error {
	return openURL(url)
}

func defaultOpen(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		fmt.Printf("Open this URL to authenticate:\n  %s\n", url)
		return err
	}
	return nil
}
