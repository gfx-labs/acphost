# Upgrading from v0.22 to v0.28

For a caller on v0.22.x moving to v0.28.0. v0.29.0 is out; its one change
is at the end. Earlier steps: `docs/upgrade-v0.6-v0.22.md`.

`Protocol` is 1 in every release from v0.22.0 to v0.29.0 (the constant in
`pkg/acphost/spec.go`). No change in this range touched spec.json's
required keys, hello/welcome, the extension methods or the journal lines,
so `Connect` and `ReadSpec` accept any v0.22 to v0.29 client with any
v0.22 to v0.29 host. The exported API of the library packages only grew:
`go doc` of `pkg/` at v0.22.5 and at v0.29.0 differ by added names
(`ConnectTCP`, `ReadListen`, `WithListening`, `WithLogger`, `Reopen` and
their types). The one thing a caller must change is the module path.

## Steps

1. Change the module path from `github.com/elee1766/acphost` to
   `github.com/gfx-labs/acphost`, in imports, `go.mod`, `go install` lines
   and any `replace` (v0.24).
2. `go get github.com/gfx-labs/acphost@v0.28.0`, then `go mod tidy`.
3. If you ran `acphost new`, `prompt`, `exec`, `attach`, `cancel`, `ls`
   or `rm` from v0.25 or v0.26, use `acpc` (v0.27). A v0.22 CLI user
   never had them.
4. Optional: the TCP listener (v0.23), `acphost.Reopen` (v0.28).

## BREAKING changes

The CHANGELOG has two BREAKING entries between v0.22.5 and v0.28.0: v0.24.0
and v0.27.0.

### v0.24.0: module path is `github.com/gfx-labs/acphost`

CHANGELOG: "move module to github.com/gfx-labs/acphost (#288)".

| | |
|---|---|
| old | `github.com/elee1766/acphost` |
| new | `github.com/gfx-labs/acphost` |
| why | the repository moved to the gfx-labs organization; the module path follows the repository |
| Protocol | 1, any mix. Go module path only, no code or wire change |

`git show v0.24.0:go.mod` is `module github.com/gfx-labs/acphost` with no
`replace` and no `retract`. `git show v0.23.0:go.mod` is still
`module github.com/elee1766/acphost`. So there is no redirect: v0.24.0 and
later exist only under the new path, and v0.23.0 and earlier only under the
old one. Go checks the path a `go.mod` declares against the path it was
required as:

```text
$ go get github.com/elee1766/acphost@v0.24.0
go: github.com/elee1766/acphost@v0.24.0 requires github.com/elee1766/acphost@v0.24.0: parsing go.mod:
	module declares its path as: github.com/gfx-labs/acphost
	        but was required as: github.com/elee1766/acphost
```

The proxy lists the same tags under both paths, so `go list -m -versions`
for the old path shows v0.24.0 to v0.29.0 too. They cannot be fetched
there. The reverse fails the same way (`github.com/gfx-labs/acphost@v0.23.0`
declares the old path). The package layout under `pkg/` is the same on
both sides of the move, so only the prefix of each import changes.

Old (v0.22):

```go
import (
	"github.com/elee1766/acphost/pkg/acphost"
	"github.com/elee1766/acphost/pkg/sandbox"
)
```

New:

```go
import (
	"github.com/gfx-labs/acphost/pkg/acphost"
	"github.com/gfx-labs/acphost/pkg/sandbox"
)
```

Do this in the caller's module:

```sh
go mod edit -droprequire github.com/elee1766/acphost
grep -rl --include='*.go' 'github.com/elee1766/acphost' . \
  | xargs sed -i 's#github.com/elee1766/acphost#github.com/gfx-labs/acphost#g'
go get github.com/gfx-labs/acphost@v0.28.0
go mod tidy
```

Then:

- A `replace github.com/elee1766/acphost => ...` in `go.mod` must be
  rewritten to the new path (`go mod edit -dropreplace` and
  `-replace github.com/gfx-labs/acphost=...`), or dropped.
- `go install github.com/elee1766/acphost/cmd/acphost@latest` in a
  Makefile, Dockerfile or CI file becomes
  `go install github.com/gfx-labs/acphost/cmd/acphost@v0.28.0`. The old
  path fails for v0.24.0 and later as above.
- `-ldflags "-X github.com/elee1766/acphost/..."`, `go:generate` lines
  and vendored or checked-in copies of the path need the same rewrite;
  `grep -rn elee1766/acphost .` finds them.
- Every module in your build that imports acphost must move. A dependency
  still on the old path builds against its own v0.23-or-earlier copy, and
  its `acphost.Spec` is a different type from yours.
- With several modules in one repository (a `go.work`), run the `go mod`
  commands in each module, and the `grep` from the root.

Running hosts are not affected. A host started by a v0.22 build keeps
running and a v0.28 client connects to it: the socket, `spec.json`,
`supervisor.lock` and the journal are the same.

### v0.27.0: session commands moved to acpc

CHANGELOG: "move named agent sessions to gfx-labs/acpc (#303)".

| | |
|---|---|
| old | `acphost new`, `prompt`, `exec`, `attach`, `cancel`, `ls`, `rm`, and `-s NAME` on `status`, `stop` and `wait` (v0.25.0 to v0.26.0 only) |
| new | the same commands in [acpc](https://github.com/gfx-labs/acpc), with the same flags |
| why | an ACP client is a separate program with its own dependencies and release cycle; acpc uses only acphost's public API (`docs/notes/2026-11-13-session-cli-to-acpc.md`) |
| Protocol | 1. No library API changed |

The session commands were added in v0.25.0 (CHANGELOG, `3dccb5b`), so
v0.22 to v0.24 had none, and this section matters only if you ran them
from v0.25.0 or v0.26.0. The sessions of those builds lived under
`$XDG_STATE_HOME/acphost/sessions` (`~/.local/state/acphost/sessions`
when it is unset; `sessionsRoot` in `cmd/acphost/session.go` at v0.26.0).
`acpc` keeps its sessions under `$XDG_STATE_HOME/acpc/sessions`. Nothing
moves the old directories. Each is a normal session dir (`spec.json`, the
journal, `supervisor.lock`), so `acphost status|stop|wait --dir DIR` still
works on one. Whether `acpc` lists or opens a directory under the old
root was not checked.

Old (v0.26):

```sh
acphost new -s review -c sandbox.yaml -- claude-code-acp
acphost prompt -s review "look at the diff"
acphost rm -s review
```

New:

```sh
acpc new -s review -c sandbox.yaml -- claude-code-acp
acpc prompt -s review "look at the diff"
acpc rm -s review
```

These `acpc` flags were checked against `acpc` v0.2.0 (`--help` of `new`,
`prompt`, `rm` and `ls`). `acpc` also has `status`, `stop` and `wait`.

What `acphost` still has: `run`, `host`, `start`, `status`, `stop` and
`wait`, on `--dir`, for debugging isolation without a controller (README
"CLI", `cmd/acphost`). Their flags are the v0.22 ones plus `--listen` and
the `--tls-*` flags (v0.23). The parser changed to `urfave/cli` in v0.25.0.
`-c`, `-v` and `--` keep their meaning, and every v0.22 flag is accepted.
The library does not import `urfave/cli`, so importing `pkg/...` does not
add it to your `go.mod`.

## Opt-in changes in v0.23 to v0.28

Not BREAKING: existing specs and calls behave as before. An older host
ignores the new spec keys.

### v0.23.0: TCP listener

| | |
|---|---|
| old | the host listens on `acp.sock` only |
| new | `Spec.Listen` (and optionally `Spec.TLS`) makes it listen on TCP too; clients use `acphost.ConnectTCP` |
| why | a controller that cannot open the session directory (another machine, a container, a pod) (`docs/notes/285-tcp-listen.md`) |
| Protocol | 1. `listen` and `tls` are new optional spec keys. A host before v0.23 ignores them and does not listen; a caller sees that from the missing `listen.json` |

```go
spec := acphost.Spec{
	Protocol: acphost.Protocol, Session: "s1", Command: "claude-code-acp",
	Dir: "/work", LeaseMS: 600_000,
	Listen: "10.0.0.5:0", // an explicit host; port 0 picks a free port
	TLS: &acphost.ListenTLS{
		Cert:     "/certs/server.crt",
		Key:      "/certs/server.key",
		ClientCA: "/certs/client-ca.crt",
		Allow:    []string{"spiffe://cluster.local/ns/jobshop/sa/controller"},
	},
}
_ = acphost.WriteSpec(dir, spec)
// ... start the host, then:
info, ok := acphost.ReadListen(dir) // info.Addr is the bound host:port
nc, br, w, err := acphost.ConnectTCP(ctx, info.Addr,
	acphost.WithClientTLSFiles(acphost.ClientTLSFiles{
		Cert: "/certs/client.crt", Key: "/certs/client.key", CA: "/certs/server-ca.crt",
	}))
```

Without `TLS` the listener is plaintext with no authentication; read
README "Without TLS" before using that. The TLS options: README "With
TLS" (mutual TLS 1.3, the `Allow` list, rotation without a restart) and
"Dead peers". All under README "TCP listener". `WithLogger` (v0.23) sets
the host's logger.

### v0.26.0: `exec` shows the ACP handshake

`acphost exec` printed the spawn, `initialize` and `session/new` steps as
they happen, as acpx does. `exec` is now `acpc exec` (see v0.27 above), so
there is nothing to adopt in acphost. It is listed because the CHANGELOG
has it under v0.26.0. The CLI sessions are described in README
"Sessions", which points to acpc.

### v0.28.0: `acphost.Reopen`

| | |
|---|---|
| old | each caller wrote its own "load the session, or summarize the journal into a new one" after a host restart |
| new | `acphost.Reopen(caller, opts)` is the one copy |
| why | the rule existed twice, in jobshop and in the old CLI (`docs/notes/306-reopen.md`) |
| Protocol | 1. Client-side helper: it talks to the agent through the connection you give it, so it works against a host of any version |

```go
res, err := acphost.Reopen(call, acphost.ReopenOptions{
	Initialize: welcome.Initialize, // skips the initialize call
	SessionID:  saved,              // "" means no earlier session
	Cwd:        "/work",
	Journal:    filepath.Join(sessionDir, acphost.JournalFile),
})
// res.SessionID is open; put res.Seed before the next prompt when it is
// not empty.
```

`call` is an `acphost.Caller`, or an `acphost.CallFunc`, with the
JSON-RPC client left to you. It must return an agent's error response as
`*acphost.RPCError`, so `Reopen` can tell a refused `session/load` from a
dead connection. README "Reopening a session".

## v0.29.0: ACP v2 turns

v0.29.0 (#310): when client and agent negotiate ACP protocol version 2
(draft), the host ends a turn on the agent's idle `state_update` and sends
`turn_end` after it (`docs/notes/2026-11-16-acp-v2-turns.md`). A caller
does not have to do anything: the change is in the host's handling of a
`protocolVersion` 2 `initialize` result (`h.v2` in `pkg/acphost/host.go`),
and a v1 session runs the same code as before. Only a client that speaks
v2 changes behavior: a successful `session/prompt` result there only says
the prompt was accepted, so end the turn on the idle `state_update` or on
the `turn_end` notification, and not on the prompt result. `Welcome.Busy`
is also true while a v2 turn runs. No wire message changed shape, so
`Protocol` stays 1.
