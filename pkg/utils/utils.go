package utils

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
)

// ProcPIDSchedStat defines the fields of a /proc/[pid]/schedstat file
// cf. https://www.kernel.org/doc/Documentation/scheduler/sched-stats.txt
type ProcPIDSchedStat struct {
	// The process ID.
	PID int
	// time spent on the cpu
	Cputime uint64
	// time spent waiting on a runqueue
	Runqueue uint64
	// # of timeslices run on this cpu
	Timeslices uint64
}

// GetProcPIDSchedStat reads and returns the schedstat for a process from the proc fs
func GetProcPIDSchedStat(procPath string, pid int) (*ProcPIDSchedStat, error) {
	stats := &ProcPIDSchedStat{PID: pid}
	schedStatPath := filepath.Join(procPath, strconv.Itoa(pid), "schedstat")
	filecontent, _ := os.ReadFile(schedStatPath)

	_, err := fmt.Fscan(
		bytes.NewBuffer(filecontent),
		&stats.Cputime,
		&stats.Runqueue,
		&stats.Timeslices,
	)

	if err != nil {
		return nil, err
	}

	return stats, err
}

// GetCmdLine reads the cmdline for a process from /proc
func GetCmdLine(procPath string, pid int) string {
	cmdLinePath := filepath.Join(procPath, strconv.Itoa(pid), "cmdline")
	filecontent, _ := os.ReadFile(cmdLinePath)
	return string(filecontent)
}

// GetProcessList reads and returns all PIDs from the proc filesystem
func GetProcessList(procFS string) []int {
	files, err := os.ReadDir(procFS)
	if err != nil {
		log.Fatal(err)
	}

	var processes []int
	for _, f := range files {
		// is it a folder?
		if !f.IsDir() {
			continue
		}
		// is the name a number?
		if pid, err := strconv.Atoi(f.Name()); err == nil {
			processes = append(processes, pid)
		}
	}

	return processes
}
