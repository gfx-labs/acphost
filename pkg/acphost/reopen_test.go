package acphost_test

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/gfx-labs/acphost/pkg/acphost"
)

// script is a Caller backed by a map of method to reply; it records the
// calls made.
type script struct {
	calls   []string
	replies map[string]func() (json.RawMessage, error)
}

func (s *script) Call(method string, _ any) (json.RawMessage, error) {
	s.calls = append(s.calls, method)
	f, ok := s.replies[method]
	if !ok {
		return nil, errors.New("unexpected call " + method)
	}
	return f()
}

func reply(v string) func() (json.RawMessage, error) {
	return func() (json.RawMessage, error) { return json.RawMessage(v), nil }
}

func fail(err error) func() (json.RawMessage, error) {
	return func() (json.RawMessage, error) { return nil, err }
}

const (
	withLoad = `{"agentCapabilities":{"loadSession":true}}`
	noLoad   = `{"agentCapabilities":{}}`
	newOK    = `{"sessionId":"new1"}`
)

func TestReopen(t *testing.T) {
	journal := filepath.Join("testdata", "journal.ndjson")
	frames, _, err := acphost.ReadJournal(journal, 0)
	if err != nil {
		t.Fatal(err)
	}
	seed := acphost.Summarize(frames, 100)
	if seed == "" {
		t.Fatal("test journal gives an empty summary")
	}
	refusal := &acphost.RPCError{Code: -32002, Message: "session not found"}
	transport := errors.New("broken pipe")

	tests := []struct {
		name      string
		init      string // initialize result handed in; "" makes Reopen call initialize
		session   string
		replies   map[string]func() (json.RawMessage, error)
		wantCalls []string
		want      acphost.ReopenResult
		wantErr   bool
		refused   bool
		fallback  bool
	}{
		{
			name: "load succeeds", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":   reply(withLoad),
				"session/load": reply(`null`),
			},
			wantCalls: []string{"initialize", "session/load"},
			want:      acphost.ReopenResult{SessionID: "old1", Loaded: true},
		},
		{
			name: "welcome carries initialize", init: withLoad, session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"session/load": reply(`null`),
			},
			wantCalls: []string{"session/load"},
			want:      acphost.ReopenResult{SessionID: "old1", Loaded: true},
		},
		{
			name: "load refused", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":   reply(withLoad),
				"session/load": fail(refusal),
				"session/new":  reply(newOK),
			},
			wantCalls: []string{"initialize", "session/load", "session/new"},
			want:      acphost.ReopenResult{SessionID: "new1", Seed: seed},
			refused:   true, fallback: true,
		},
		{
			name: "refusal wrapped by the caller", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":   reply(withLoad),
				"session/load": fail(errors.Join(errors.New("call"), refusal)),
				"session/new":  reply(newOK),
			},
			wantCalls: []string{"initialize", "session/load", "session/new"},
			want:      acphost.ReopenResult{SessionID: "new1", Seed: seed},
			refused:   true, fallback: true,
		},
		{
			name: "no loadSession capability", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":  reply(noLoad),
				"session/new": reply(newOK),
			},
			wantCalls: []string{"initialize", "session/new"},
			want:      acphost.ReopenResult{SessionID: "new1", Seed: seed},
			fallback:  true,
		},
		{
			name: "transport error on load", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":   reply(withLoad),
				"session/load": fail(transport),
			},
			wantCalls: []string{"initialize", "session/load"},
			wantErr:   true,
		},
		{
			name: "transport error on initialize", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize": fail(transport),
			},
			wantCalls: []string{"initialize"},
			wantErr:   true,
		},
		{
			name: "no earlier session",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":  reply(withLoad),
				"session/new": reply(newOK),
			},
			wantCalls: []string{"initialize", "session/new"},
			want:      acphost.ReopenResult{SessionID: "new1"},
		},
		{
			name: "new without sessionId", session: "old1",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":  reply(noLoad),
				"session/new": reply(`{}`),
			},
			wantCalls: []string{"initialize", "session/new"},
			wantErr:   true, fallback: true,
		},
		{
			name: "new refused", session: "",
			replies: map[string]func() (json.RawMessage, error){
				"initialize":  reply(noLoad),
				"session/new": fail(refusal),
			},
			wantCalls: []string{"initialize", "session/new"},
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &script{replies: tt.replies}
			var refused, fellBack bool
			got, err := acphost.Reopen(s, acphost.ReopenOptions{
				Initialize:   json.RawMessage(tt.init),
				SessionID:    tt.session,
				Cwd:          "/work",
				Journal:      journal,
				SummaryBytes: 100,
				OnRefused: func(err error) {
					refused = true
					if !acphost.IsRPCError(err) {
						t.Errorf("OnRefused got %v, not an RPC error", err)
					}
				},
				OnFallback: func() { fellBack = true },
			})
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, want error: %v", err, tt.wantErr)
			}
			if strings.Contains(tt.name, "transport") && !errors.Is(err, transport) {
				t.Errorf("err = %v, want the transport error unchanged", err)
			}
			if !slices.Equal(s.calls, tt.wantCalls) {
				t.Errorf("calls = %v, want %v", s.calls, tt.wantCalls)
			}
			if got != tt.want {
				t.Errorf("result = %+v, want %+v", got, tt.want)
			}
			if refused != tt.refused || fellBack != tt.fallback {
				t.Errorf("refused, fallback = %v, %v, want %v, %v", refused, fellBack, tt.refused, tt.fallback)
			}
		})
	}
}

func TestReopenNoJournal(t *testing.T) {
	for _, path := range []string{"", filepath.Join(t.TempDir(), "journal.ndjson")} {
		s := &script{replies: map[string]func() (json.RawMessage, error){
			"initialize":  reply(noLoad),
			"session/new": reply(newOK),
		}}
		got, err := acphost.Reopen(s, acphost.ReopenOptions{SessionID: "old1", Journal: path})
		if err != nil {
			t.Fatal(err)
		}
		if got.SessionID != "new1" || got.Seed != "" || got.Loaded {
			t.Errorf("result = %+v", got)
		}
	}
}
