package sys

import (
	"os"
	"syscall"
)

func SetHighPriority() error {
	return syscall.Setpriority(syscall.PRIO_PROCESS, os.Getpid(), -10)
}
