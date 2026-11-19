package children

import (
	"os"
	"os/exec"
	"strconv"
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

func itoa(n int) string { return string(appendInt(nil, n)) }

func appendInt(b []byte, n int) []byte {
	if n >= 10 {
		b = appendInt(b, n/10)
	}
	return append(b, byte('0'+n%10))
}

// Reap leaves a child that Start started for its Cmd, and reaps one
// that nothing waits for.
func TestReap(t *testing.T) {
	require.NoError(t, unix.Prctl(unix.PR_SET_CHILD_SUBREAPER, 1, 0, 0, 0))

	tracked := exec.Command("true")
	require.NoError(t, Start(tracked))

	// An orphan: the subshell exits at once, and the sleep reparents to us.
	orphan := exec.Command("sh", "-c", "(sleep 0.1 & echo $!) ")
	out, err := orphan.Output()
	require.NoError(t, err)
	var opid int
	for _, c := range out {
		if c >= '0' && c <= '9' {
			opid = opid*10 + int(c-'0')
		}
	}
	require.NotZero(t, opid)

	// Both are zombies after a while; the tracked one must survive.
	require.Eventually(t, func() bool {
		Reap()
		return gone(opid)
	}, 5*time.Second, 20*time.Millisecond)
	time.Sleep(50 * time.Millisecond)
	Reap()
	assert.NoError(t, tracked.Wait(), "Wait lost its child to the reaper")
}
