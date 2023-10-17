package cpulimitor

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestCpulimitor(t *testing.T) {
	childExeName := "./test"
	if runtime.GOOS == "windows" {
		childExeName = ".\\test.exe"
	}
	cmd := exec.Command("go", "build", "../../cmd/child/test.go")
	err := cmd.Run()
	if err != nil {
		t.Errorf("build child process failed,err:%v", err)
	}

	ctx, cancel := context.WithTimeout(context.TODO(), 3*time.Minute)

	defer func() {
		cancel()
		time.Sleep(time.Second * 2)
		os.Remove(childExeName)
	}()

	limit := 1.2
	lazy := true
	includeChildren := true
	duration := 100 * time.Millisecond
	pid := 0
	name := ""
	command := childExeName

	New(limit, lazy, includeChildren, duration, pid, name, command).Run(ctx)
}
