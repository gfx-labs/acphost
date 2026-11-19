package acphost

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The host is a subreaper, so what the agent leaves behind reparents to
// it. It must reap those (#324) and still report the agent's own exit.
func TestHostReapsOrphans(t *testing.T) {
	dir := t.TempDir()
	agent := filepath.Join(dir, "agent")
	orphan := filepath.Join(dir, "orphan.pid")
	agentPid := filepath.Join(dir, "agent.pid")
	script := "#!/bin/sh\n" +
		"echo $$ > " + agentPid + "\n" +
		"(sleep 0.3 & echo $! > " + orphan + ")\n" +
		"exec sleep 30\n"
	require.NoError(t, os.WriteFile(agent, []byte(script), 0o700))
	require.NoError(t, WriteSpec(dir, Spec{Protocol: Protocol, Session: "s1", Command: agent, Dir: dir, GCRootDir: dir}))
	done := make(chan error, 1)
	go func() { done <- Main(dir) }()

	read := func(p string) int {
		var pid int
		require.Eventually(t, func() bool {
			b, err := os.ReadFile(p)
			if err != nil {
				return false
			}
			pid, err = strconv.Atoi(strings.TrimSpace(string(b)))
			return err == nil
		}, 5*time.Second, 10*time.Millisecond)
		return pid
	}
	opid := read(orphan)
	apid := read(agentPid)

	// A zombie keeps its /proc entry until its parent waits for it.
	require.Eventually(t, func() bool {
		_, err := os.Stat("/proc/" + strconv.Itoa(opid))
		return os.IsNotExist(err)
	}, 10*time.Second, 50*time.Millisecond, "orphan %d was not reaped", opid)

	require.NoError(t, syscall.Kill(apid, syscall.SIGKILL))
	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(15 * time.Second):
		t.Fatal("host did not exit")
	}
	ex, ok := ReadExit(dir)
	require.True(t, ok)
	assert.Equal(t, "killed", ex.Signal, "the agent's exit status was lost")
}
