//go:build !windows

package procgroup

import (
	"os/exec"
	"syscall"
)

func SetProcGrp(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}