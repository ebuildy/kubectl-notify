//go:build unix

package daemon

import "syscall"

// detachSysProcAttr starts the child in a new session so it detaches from the
// controlling terminal and survives the parent's exit.
func detachSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}
