package cpulimitor

import (
	"context"
	"cpulimit/pkg/logger"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/shirou/gopsutil/process"
)

const (
	MINDT = 20
	ALFA  = 0.08
)

type CpuLimiter struct {
	logger          logger.Logger
	percent         float64
	lazy            bool
	includeChildren bool
	pid             int
	name            string
	command         string
	duration        time.Duration

	lastUpdate time.Time
	procs      map[int]*ProcInfo
}

type ProcInfo struct {
	pid       int
	cmdline   string
	startTime time.Time
	cputime   float64
	cpuUsage  float64
	parent    *ProcInfo
	children  []*ProcInfo
}

func New(pencent float64, lazy, includeChildren bool, duration time.Duration, pid int, name, cmd string) *CpuLimiter {
	if duration <= 0 {
		duration = 100 * time.Millisecond
	}
	return &CpuLimiter{
		percent:         pencent,
		lazy:            lazy,
		includeChildren: includeChildren,
		pid:             pid,
		name:            name,
		command:         cmd,
		duration:        duration,

		procs: make(map[int]*ProcInfo, 0),
	}
}

func (c *CpuLimiter) WithLogger(logger logger.Logger) {
	c.logger = logger
}

func (c *CpuLimiter) Run(ctx context.Context) error {
	if c.logger == nil {
		c.logger = logger.Default()
	}

	if c.command == "" && c.pid <= 0 && c.name == "" {
		return fmt.Errorf("at least one of command,pid and exe should provide")
	}

	for {
		if err := c.attach(); err != nil {
			c.logger.Errorf("attach process failed,err:%v", err)
			if c.lazy {
				return err
			} else {
				time.Sleep(2 * time.Second)
				continue
			}
		}

		if err := c.limit(ctx); err != nil {
			c.logger.Errorf("start limiter failed,err:%v", err)
			if c.lazy {
				return err
			} else {
				time.Sleep(2 * time.Second)
				continue
			}
		} else {
			// ctx done
			return nil
		}
	}
}

func (c *CpuLimiter) attach() error {
	if c.command != "" {
		// parse command and run
		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			cmd = exec.Command("cmd", "/c", c.command)
		} else {
			cmd = exec.Command("bash", "-c", c.command)
		}
		err := cmd.Start()
		if err != nil {
			return err
		}

		c.pid = cmd.Process.Pid

		go func() {
			err := cmd.Wait()
			c.logger.Warnf("process '%s' exit with err:%v", c.command, err)
		}()

		return nil
	} else if c.pid > 0 {
		// find process of pid
		_, err := process.NewProcess(int32(c.pid))
		if err != nil {
			return err
		}
		return nil
	} else {
		// find process by name
		pids, err := process.Pids()
		if err != nil {
			return err
		}
		for _, pid := range pids {
			proc, err := process.NewProcess(pid)
			if err != nil {
				continue
			}
			name, err := proc.Name()
			if err != nil {
				continue
			}
			if name == c.name {
				c.pid = int(pid)
				break
			}
		}
		return fmt.Errorf("process not found by name %s", c.name)
	}
}

func (c *CpuLimiter) resume() {
	// resume processes
	for pid, _ := range c.procs {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			c.logger.Debugf("proc of pid %d is not exist", pid)
			delete(c.procs, pid)
			continue
		}
		err = proc.Resume()
		if err != nil {
			c.logger.Debugf("send SIGCONT signal to proc %d failed,err:%v", pid, err)
			delete(c.procs, pid)
			continue
		}
	}
}

func (c *CpuLimiter) suspend() {
	for pid, _ := range c.procs {
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			c.logger.Debugf("proc of pid %d is not exist", pid)
			delete(c.procs, pid)
			continue
		}
		err = proc.Suspend()
		if err != nil {
			c.logger.Debugf("send SIGCONT signal to proc %d failed,err:%v", pid, err)
			delete(c.procs, pid)
			continue
		}
	}
}

func (c *CpuLimiter) kill() {
	proc, err := process.NewProcess(int32(c.pid))
	if err != nil {
		return
	}
	proc.Kill()
}

var cpuTotal float64 = 0.0
var workTotal time.Duration = 0
var sleepTotal time.Duration = 0

func (c *CpuLimiter) limit(ctx context.Context) error {
	// percent of working
	workingrate := -1.0
	// time to work
	twork := time.Second * 0
	// time should sleep
	tsleep := time.Second * 0
	count := 0

	// make sure all processes are resumed when limiter exit
	defer c.resume()

	for {
		select {
		case <-ctx.Done():
			c.logger.Debugf("context done")
			if c.command != "" {
				c.logger.Warnf("will kill process from command :%s", c.command)
				c.kill()
			}

			return nil
		default:
			err := c.updateProcessGroup()
			if err != nil || len(c.procs) == 0 {
				c.logger.Warnf("no process")
				return fmt.Errorf("no process")
			}

			// calculate all processes cpu percent
			pcpu := -1.0
			for _, v := range c.procs {
				if v.cpuUsage < 0 {
					continue
				}
				if pcpu < 0 {
					pcpu = 0
				}
				pcpu += v.cpuUsage
			}

			// calculate time to work and sleep
			if pcpu <= 0 {
				pcpu = c.percent
				workingrate = c.percent
				twork = time.Duration(float64(c.duration) * c.percent)
			} else {
				workingrate = workingrate / pcpu * c.percent
				if workingrate > 1 {
					workingrate = 1
				}
				twork = time.Duration(float64(c.duration) * workingrate)
			}

			tsleep = c.duration - twork
			if count%200 == 0 {
				c.logger.Debugf("CPU\twork quantum\tsleep quantum\tactive rate")
			}
			if count%10 == 0 && count > 0 {
				c.logger.Debugf("%0.2f%%\t%6d ms\t%6d ms\t%0.2f%%",
					cpuTotal/10*100,
					(workTotal / 10).Milliseconds(),
					(sleepTotal / 10).Milliseconds(),
					workingrate*100)

				cpuTotal = 0.0
				workTotal = 0
				sleepTotal = 0
			} else {
				cpuTotal += pcpu
				workTotal += twork
				sleepTotal += tsleep
			}

			// resume all process and wrok
			c.resume()
			start := time.Now()
			time.Sleep(twork)
			workingtime := time.Since(start)

			delay := workingtime - tsleep
			if count > 100000 && delay > time.Millisecond*15 {
				c.logger.Debugf("delay too much,count: %d,twork: %d ms,tsleep: %d ms,wroking: %d ms,overtime: %d ms,delay: %d ms",
					count,
					twork.Milliseconds(),
					tsleep.Milliseconds(),
					workingtime.Milliseconds(),
					(workingtime - twork).Milliseconds(),
					delay.Milliseconds())
			}

			// stop processes only if tsleep > 0
			if tsleep > 0 {
				c.suspend()
				time.Sleep(tsleep)
			}
			count++
		}
	}
}

func (c *CpuLimiter) updateProcessGroup() error {
	proc, err := process.NewProcess(int32(c.pid))
	if err != nil {
		c.logger.Errorf("New process failed,pid:%d,err:%v", c.pid, err)
		return err
	}

	_, err = c.updateProcInfo(proc, nil)
	if err != nil {
		c.logger.Warnf("get process info of pid:%d failed,err:%v", c.pid, err)
		return err
	}
	c.lastUpdate = time.Now()

	return nil
}

func (c *CpuLimiter) updateProcInfo(proc *process.Process, parent *ProcInfo) (*ProcInfo, error) {
	info := &ProcInfo{
		pid:      int(proc.Pid),
		parent:   parent,
		children: make([]*ProcInfo, 0),
		cpuUsage: -1.0,
		cputime:  -1.0,
	}
	cmdline, err := proc.Cmdline()
	if err != nil {
		c.logger.Warnf("get process(pid:%d) cmdline failed,err:%v", proc.Pid, err)
	}
	info.cmdline = cmdline

	creatTime, err := proc.CreateTime()
	if err != nil {
		c.logger.Warnf("get process(pid:%d) create time failed,err:%v", proc.Pid, err)
		return nil, err
	}
	info.startTime = time.Unix(0, creatTime*int64(time.Millisecond))

	// get cpu time
	times, err := proc.Times()
	if err != nil {
		c.logger.Errorf("get process(pid:%d) time failed,err:%v", proc.Pid, err)
		return nil, err
	}
	info.cputime = (times.User + times.System) * float64(time.Second) // in ns

	// calculate cpu percent in difftime (ns)
	difftime := time.Since(c.lastUpdate).Nanoseconds()
	if v, ok := c.procs[info.pid]; ok {
		if difftime < MINDT {
			return info, nil
		}

		// cputime used in ns
		use := info.cputime - v.cputime
		usage := use / float64(difftime)
		if v.cpuUsage < 0 {
			v.cpuUsage = usage
		} else {
			v.cpuUsage = (1.0-ALFA)*v.cpuUsage + ALFA*usage
		}
		v.cputime = info.cputime
	} else {
		c.procs[info.pid] = info
	}
	// update children
	if c.includeChildren {
		children, err := proc.Children()
		if err != nil {
			return info, nil
		}

		for _, child := range children {
			childInfo, err := c.updateProcInfo(child, info)
			if err != nil {
				continue
			}
			info.children = append(info.children, childInfo)
		}
	}

	return info, nil
}
