# #306 Reopen: load or summarize after a host restart

2026-11-16

The rule was written twice (jobshop `control/session.go`, the old acphost
CLI now in acpc). `acphost.Reopen` is the one copy.

- The library has no JSON-RPC client, so `Reopen` takes a `Caller`
  (`Call(method, params) (json.RawMessage, error)`). The caller keeps ids,
  muting of replayed output, logging and persisted state.
- A JSON-RPC error response must come back as `*RPCError` (or wrap one;
  `IsRPCError` checks). Any other error is a transport error.
- Steps: `initialize` (skipped when `Initialize` is given, e.g. from the
  welcome line), then `session/load` if `agentCapabilities.loadSession`.
- Load succeeds: same id, `Loaded`, no seed.
- Load refused (RPC error) or no capability: `OnRefused` (only for a
  refusal) and `OnFallback`, then `session/new`; `Seed` is
  `Summarize(ReadJournal(Journal), SummaryBytes)`. The load is not retried
  later: it would fail again after every restart.
- Transport error on any call: returned, no fallback.
- Empty `SessionID`: `session/new`, no seed. `session/new` without a
  `sessionId`: error.
- A missing or unreadable journal gives an empty seed, not an error.
- `mcpServers` is sent as `[]`.
- Not covered: the old CLI's "same host pid, nothing to reopen" shortcut.
  The caller decides whether to call `Reopen` at all.
