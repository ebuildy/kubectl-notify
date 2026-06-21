//go:build !unix

package daemon

import "syscall"

// detachSysProcAttr is best-effort on non-Unix platforms: there is no portable
// session-detach knob here, so the child inherits the default attributes.
func detachSysProcAttr() *syscall.SysProcAttr {
	return nil
}
