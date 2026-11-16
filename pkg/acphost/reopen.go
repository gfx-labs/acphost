package acphost

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Caller sends one JSON-RPC request to the agent and returns the result.
// The caller owns the connection, request ids, and what to do with the
// output the agent sends meanwhile (a session/load replays the whole
// conversation, which a UI usually mutes).
//
// An error response from the agent must be returned as an *RPCError (or
// wrap one). Any other error is a transport error.
type Caller interface {
	Call(method string, params any) (json.RawMessage, error)
}

// CallFunc makes a function a Caller.
type CallFunc func(method string, params any) (json.RawMessage, error)

// Call calls f.
func (f CallFunc) Call(method string, params any) (json.RawMessage, error) {
	return f(method, params)
}

// RPCError is a JSON-RPC error response from the agent. Reopen treats it
// as the agent refusing the request. Every other error from a Caller is a
// transport error and is returned as it is.
type RPCError struct {
	Code    int
	Message string
	Data    json.RawMessage
}

func (e *RPCError) Error() string {
	return fmt.Sprintf("rpc error %d: %s", e.Code, e.Message)
}

// IsRPCError reports whether err is or wraps an *RPCError.
func IsRPCError(err error) bool {
	var e *RPCError
	return errors.As(err, &e)
}

// ReopenOptions says what Reopen has to work with.
type ReopenOptions struct {
	// Initialize is the agent's initialize result when the caller already
	// has it (Welcome.Initialize). Reopen then skips the initialize call.
	Initialize json.RawMessage
	// ClientName and ClientVersion go in clientInfo of the initialize
	// call. ClientName defaults to "acphost".
	ClientName, ClientVersion string
	// SessionID is the ACP session to reload. Empty means there is none
	// yet: Reopen goes straight to session/new.
	SessionID string
	// Cwd is the session's working directory.
	Cwd string
	// Journal is the path of the session's journal, read for the summary
	// when the conversation cannot be reloaded.
	Journal string
	// SummaryBytes caps the summary, 0 means DefaultSummaryBytes.
	SummaryBytes int
	// OnRefused is called with the agent's error when it refuses
	// session/load, before the fallback.
	OnRefused func(err error)
	// OnFallback is called when Reopen starts a new session in place of
	// the one it could not reload, before session/new.
	OnFallback func()
}

// ReopenResult is what Reopen opened.
type ReopenResult struct {
	// SessionID is the ACP session now open.
	SessionID string
	// Loaded is true when the earlier session was loaded.
	Loaded bool
	// Seed is a summary of the journal, to put in front of the next
	// prompt. Set only when a new session replaced an earlier one.
	Seed string
}

// acpProtocolVersion is the ACP protocol version Reopen announces.
const acpProtocolVersion = 1

// Reopen opens an ACP session on a freshly started agent, the one a host
// restart leaves behind: initialize (unless o.Initialize is set), then
// session/load when the agent advertises agentCapabilities.loadSession.
// If the agent cannot or will not load it (no capability, or a JSON-RPC
// error), it falls back to session/new and returns Summarize of the
// journal as Seed. The load is not retried: a refused load would be
// refused again after every restart. A transport error is returned, not
// turned into a fallback. An empty o.SessionID means no earlier session:
// session/new with no seed.
func Reopen(c Caller, o ReopenOptions) (ReopenResult, error) {
	var caps struct {
		AgentCapabilities struct {
			LoadSession bool `json:"loadSession"`
		} `json:"agentCapabilities"`
	}
	init := o.Initialize
	if len(init) == 0 {
		var err error
		if init, err = initialize(c, o); err != nil {
			return ReopenResult{}, err
		}
	}
	// An unreadable result means no capabilities: start over.
	_ = json.Unmarshal(init, &caps)
	if o.SessionID == "" {
		id, err := newSession(c, o.Cwd)
		return ReopenResult{SessionID: id}, err
	}
	if caps.AgentCapabilities.LoadSession {
		_, err := c.Call("session/load", sessionParams{SessionID: o.SessionID, Cwd: o.Cwd, McpServers: []string{}})
		if err == nil {
			return ReopenResult{SessionID: o.SessionID, Loaded: true}, nil
		}
		if !IsRPCError(err) {
			return ReopenResult{}, err
		}
		if o.OnRefused != nil {
			o.OnRefused(err)
		}
	}
	if o.OnFallback != nil {
		o.OnFallback()
	}
	// A journal that cannot be read gives no seed: the new session is
	// still worth opening.
	frames, _, _ := ReadJournal(o.Journal, 0)
	seed := Summarize(frames, o.SummaryBytes)
	id, err := newSession(c, o.Cwd)
	if err != nil {
		return ReopenResult{}, err
	}
	return ReopenResult{SessionID: id, Seed: seed}, nil
}

type sessionParams struct {
	SessionID  string   `json:"sessionId,omitempty"`
	Cwd        string   `json:"cwd"`
	McpServers []string `json:"mcpServers"`
}

func initialize(c Caller, o ReopenOptions) (json.RawMessage, error) {
	name := o.ClientName
	if name == "" {
		name = "acphost"
	}
	type info struct {
		Name    string `json:"name"`
		Version string `json:"version,omitempty"`
	}
	return c.Call("initialize", struct {
		ProtocolVersion int  `json:"protocolVersion"`
		ClientInfo      info `json:"clientInfo"`
	}{acpProtocolVersion, info{name, o.ClientVersion}})
}

func newSession(c Caller, cwd string) (string, error) {
	res, err := c.Call("session/new", sessionParams{Cwd: cwd, McpServers: []string{}})
	if err != nil {
		return "", err
	}
	var r struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(res, &r); err != nil || r.SessionID == "" {
		return "", fmt.Errorf("session/new: no sessionId in %s", res)
	}
	return r.SessionID, nil
}
