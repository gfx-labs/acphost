// Package children keeps the host from leaving zombies. The host is a
// child subreaper, so a process the agent leaves behind reparents to
// it, and nobody waits for it. Reap does, but only for pids that no
// exec.Cmd in this process waits for: a Wait4 on those would make the
// Cmd's own Wait fail with ECHILD and lose the exit status.
//
// Every command the library starts goes through Start or Run, which
// record its pid. Anything else that is a zombie child of this process
// is an orphan.
package children

import (
	"errors"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/sys/unix"
)

var (
	// mu orders spawning against reaping: Start holds it shared from
	// before the fork until the pid is recorded, Reap holds it exclusively,
	// so Reap never sees a child that is started but not yet recorded.
	mu      sync.RWMutex
	trackMu sync.Mutex
	tracked = map[int]uint64{} // pid to process start time
)

// Start starts cmd and records its pid as one the caller waits for.
func Start(cmd *exec.Cmd) error {
	mu.RLock()
	defer mu.RUnlock()
	if err := cmd.Start(); err != nil {
		return err
	}
	_, _, started := procState(cmd.Process.Pid)
	trackMu.Lock()
	tracked[cmd.Process.Pid] = started
	trackMu.Unlock()
	return nil
}

// Run is Start followed by Wait.
func Run(cmd *exec.Cmd) error {
	if err := Start(cmd); err != nil {
		return err
	}
	return cmd.Wait()
}

// prune forgets pids that are gone, to bound the map.
func prune() {
	trackMu.Lock()
	defer trackMu.Unlock()
	for pid := range tracked {
		if err := unix.Kill(pid, 0); errors.Is(err, syscall.ESRCH) {
			delete(tracked, pid)
		}
	}
}

// isTracked reports whether pid is a process Start started. A Cmd that
// was waited for leaves its pid in the map until prune, and the kernel
// may give that pid to an orphan since, so the start time must match too.
func isTracked(pid int) bool {
	trackMu.Lock()
	defer trackMu.Unlock()
	want, ok := tracked[pid]
	if !ok {
		return false
	}
	if _, _, got := procState(pid); want != 0 && got != 0 && got != want {
		delete(tracked, pid)
		return false
	}
	return true
}

// Reap waits for every zombie child of this process that Start did not
// start, and returns how many it reaped.
func Reap() int {
	mu.Lock()
	defer mu.Unlock()
	prune()
	n := 0
	for _, pid := range zombieChildren() {
		if isTracked(pid) {
			continue
		}
		var ws unix.WaitStatus
		// ECHILD: someone else (a test's own Cmd) got it first.
		if wpid, err := unix.Wait4(pid, &ws, unix.WNOHANG, nil); err == nil && wpid == pid {
			n++
		}
	}
	return n
}

// zombieChildren lists the zombie children of this process.
func zombieChildren() []int {
	pids, ok := childPids()
	if !ok {
		pids = scanChildren()
	}
	var z []int
	for _, pid := range pids {
		if st, _, _ := procState(pid); st == 'Z' {
			z = append(z, pid)
		}
	}
	return z
}

// childPids reads /proc/self/task/*/children, which a kernel without
// CONFIG_PROC_CHILDREN does not have (ok is false then).
func childPids() (pids []int, ok bool) {
	tasks, err := os.ReadDir("/proc/self/task")
	if err != nil {
		return nil, false
	}
	for _, t := range tasks {
		b, err := os.ReadFile("/proc/self/task/" + t.Name() + "/children")
		if err != nil {
			if os.IsNotExist(err) && len(pids) == 0 {
				return nil, false
			}
			continue // the thread exited
		}
		for _, f := range strings.Fields(string(b)) {
			if pid, err := strconv.Atoi(f); err == nil {
				pids = append(pids, pid)
			}
		}
	}
	return pids, true
}

// scanChildren is the slow fallback: every pid in /proc whose parent
// is this process.
func scanChildren() []int {
	ents, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	var pids []int
	me := os.Getpid()
	for _, e := range ents {
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		if _, ppid, _ := procState(pid); ppid == me {
			pids = append(pids, pid)
		}
	}
	return pids
}

// procState returns the state letter and parent pid from
// /proc/pid/stat, and the start time in clock ticks since boot. It
// returns 0, -1 and 0 when the file cannot be read.
func procState(pid int) (state byte, ppid int, started uint64) {
	b, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return 0, -1, 0
	}
	// The name is in parentheses and may hold spaces and parentheses.
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		return 0, -1, 0
	}
	f := strings.Fields(string(b[i+1:]))
	if len(f) < 2 {
		return 0, -1, 0
	}
	ppid, err = strconv.Atoi(f[1])
	if err != nil {
		return 0, -1, 0
	}
	// Field 22 of the line; f starts at field 3.
	if len(f) > 19 {
		started, _ = strconv.ParseUint(f[19], 10, 64)
	}
	return f[0][0], ppid, started
}
