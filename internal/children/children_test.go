package children

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func gone(pid int) bool {
	_, err := os.Stat("/proc/" + strconv.Itoa(pid))
	return os.IsNotExist(err)
}

// Reap leaves a child that Start started for its Cmd, and reaps one
// that nothing waits for.
func TestReap(t *testing.T) {
	require.NoError(t, unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0))

	tracked := exec.Command("sh", "-c", "exit 0")
	require.NoError(t, Start(tracked))

	// An orphan: the subshell exits at once, and the sleep reparents to us.
	orphan := exec.Command("sh", "-c", "(sleep 0.1 & echo $!) ")
	out, err := orphan.Output()
	require.NoError(t, err)
	opid, err := strconv.Atoi(strings.TrimSpace(string(out)))
	require.NoError(t, err)

	// Both are zombies after a while; the tracked one must survive.
	require.Eventually(t, func() bool {
		Reap()
		return gone(opid)
	}, 5*time.Second, 20*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	Reap()
	assert.NoError(t, tracked.Wait(), "Wait lost its child to the reaper")
}

// A recorded pid that the kernel gave to another process is not tracked.
func TestTrackedPidReuse(t *testing.T) {
	pid := os.Getpid()
	_, _, started := procState(pid)
	require.NotZero(t, started)
	trackMu.Lock()
	tracked[pid] = started + 1
	trackMu.Unlock()
	assert.False(t, isTracked(pid))
	trackMu.Lock()
	tracked[pid] = started
	trackMu.Unlock()
	assert.True(t, isTracked(pid))
	trackMu.Lock()
	delete(tracked, pid)
	trackMu.Unlock()
}
