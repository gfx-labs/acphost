package acphost_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gfx-labs/acphost/pkg/acphost"
)

// With ACP v2 negotiated, a prompt's result only acknowledges it. The host
// keeps the session busy until the idle state_update and reports that as
// turn_end, with its stop reason, to a controller that was not connected.
func TestV2TurnEndsOnIdleState(t *testing.T) {
	dir := t.TempDir()
	agent := writeAgent(t, dir, `read i
echo '{"jsonrpc":"2.0","id":"h1","result":{"protocolVersion":2}}'
read p
echo '{"jsonrpc":"2.0","id":"h2","result":{"messageId":"m1"}}'
read go
echo '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"hi"}}}}'
echo '{"jsonrpc":"2.0","method":"session/update","params":{"sessionId":"s","update":{"sessionUpdate":"state_update","state":"idle","stopReason":"end_turn"}}}'
exec sleep 600
`)
	h, _, _ := startListening(t, acphost.Spec{Command: agent, LeaseMS: 60_000})

	nc, rd, _, err := acphost.Connect(t.Context(), h.Dir)
	require.NoError(t, err)
	// As a client does, the prompt waits for the initialize result.
	for _, req := range []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":2}}`,
		`{"jsonrpc":"2.0","id":2,"method":"session/prompt","params":{}}`,
	} {
		_, err = nc.Write([]byte(req + "\n"))
		require.NoError(t, err)
		_, err = rd.ReadBytes('\n')
		require.NoError(t, err)
	}
	require.NoError(t, nc.Close())

	nc2, rd2, w, err := acphost.Connect(t.Context(), h.Dir)
	require.NoError(t, err)
	t.Cleanup(func() { _ = nc2.Close() })
	assert.True(t, w.Busy, "acknowledged v2 prompt is still running")
	_, err = nc2.Write([]byte(`{"go":true}` + "\n"))
	require.NoError(t, err)

	var methods []string
	for {
		line, err := rd2.ReadBytes('\n')
		require.NoError(t, err)
		var f struct {
			Method string          `json:"method"`
			Params acphost.TurnEnd `json:"params"`
		}
		require.NoError(t, json.Unmarshal(line, &f))
		methods = append(methods, f.Method)
		if f.Method == acphost.MethodTurnEnd {
			assert.JSONEq(t, `{"stopReason":"end_turn"}`, string(f.Params.Result))
			break
		}
	}
	assert.Equal(t, []string{"session/update", "session/update", acphost.MethodTurnEnd}, methods)
}
