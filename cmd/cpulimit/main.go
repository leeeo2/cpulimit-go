package main

import (
	"context"
	"cpulimit/pkg/cpulimitor"
	"cpulimit/pkg/sys"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	_ "github.com/shirou/gopsutil/process"
)

func main() {
	limit := flag.Int("limit", 150, fmt.Sprintf("percentage of cpu allowed from 0 to %d (required)", runtime.NumCPU()*100))
	lazy := flag.Bool("lazy", false, "exit if there is no target process,or if it dies")
	includeChildren := flag.Bool("include-children", false, "limit also the children processes")
	duration := flag.Int("duration", 100, "cpu sample duration in ms")
	pid := flag.Int("pid", 0, "pid of the process (implies lazy)")
	name := flag.String("name", "", "process name,limit only the first one if there are more than one(implies lazy)")
	cmd := flag.String("command", "", "run this command and limit it (implies lazy)")

	flag.Parse()

	fmt.Printf("limit: %d \nlazy: %v \ninclude children: %v \nduration: %d ms\npid: %d \nname: %s \ncmd: %s \n",
		*limit, *lazy, *includeChildren, *duration, *pid, *name, *cmd)

	// up process priority
	if err := sys.SetHighPriority(); err != nil {
		fmt.Println("set high priority failed,err:", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		ch := make(chan os.Signal, 1)
		signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
		sig := <-ch
		log.Printf("signal %s recived,cpulimit exit", sig.String())
		cancel()
	}()

	dur:=time.Duration(*duration)*time.Millisecond
	limitor := cpulimitor.New(float64(*limit), *lazy, *includeChildren,dur, *pid, *name, *cmd)
	limitor.Run(ctx)
}
