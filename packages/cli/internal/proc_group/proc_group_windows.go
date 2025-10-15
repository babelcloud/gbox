//go:build windows

package procgroup

import (
	"os/exec"
	"syscall"
)

func SetProcGrp(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
