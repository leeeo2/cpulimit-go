package sys

import (
	"golang.org/x/sys/windows"
)

func SetHighPriority() error {
	p := windows.CurrentProcess()
	return windows.SetPriorityClass(p, windows.HIGH_PRIORITY_CLASS)
}
