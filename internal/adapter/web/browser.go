package web

import (
	"fmt"
	"os/exec"
	"runtime"
)

// OpenBrowser attempts to open url in the user's default browser, using the
// platform-native opener. It returns an error on an unsupported platform or if
// the opener cannot be launched; callers may treat failure as non-fatal (the
// URL is always also printed).
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("web: cannot open browser on %s", runtime.GOOS)
	}
	return cmd.Start()
}
