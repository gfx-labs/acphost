package acphost_test

import (
	"encoding/json"
	"fmt"

	"github.com/gfx-labs/acphost/pkg/acphost"
)

// Reopen opens an ACP session on a restarted agent: it loads the earlier
// one when the agent can, and otherwise starts a new one and returns a
// summary of the journal to put in front of the next prompt. The Caller
// here is a stub; a real one sends the request over the host socket and
// returns an *acphost.RPCError for an error response.
func ExampleReopen() {
	var c acphost.CallFunc = func(method string, _ any) (json.RawMessage, error) {
		switch method {
		case "initialize":
			return json.RawMessage(`{"agentCapabilities":{"loadSession":true}}`), nil
		case "session/load":
			return nil, &acphost.RPCError{Code: -32002, Message: "session not found"}
		default: // session/new
			return json.RawMessage(`{"sessionId":"s2"}`), nil
		}
	}
	res, err := acphost.Reopen(c, acphost.ReopenOptions{
		SessionID: "s1",
		Cwd:       "/work",
		Journal:   "testdata/journal.ndjson",
		OnRefused: func(err error) { fmt.Println("refused:", err) },
	})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(res.SessionID, res.Loaded, res.Seed != "")
	// Output:
	// refused: rpc error -32002: session not found
	// s2 false true
}
