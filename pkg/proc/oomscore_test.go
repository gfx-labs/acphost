package proc

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRaiseOOMScore(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	raiseOOMScore(cmd.Process.Pid)
	b, err := os.ReadFile("/proc/" + strconv.Itoa(cmd.Process.Pid) + "/oom_score_adj")
	require.NoError(t, err)
	assert.Equal(t, strconv.Itoa(agentOOMScoreAdj), strings.TrimSpace(string(b)))
	// Already at or above: left alone, and no error for a gone pid.
	raiseOOMScore(cmd.Process.Pid)
	raiseOOMScore(1 << 30)
}
