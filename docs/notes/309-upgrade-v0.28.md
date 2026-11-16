# #309 upgrade guide v0.22 to v0.28

2026-11-16. Items 1 and 2 of #309, issue #312. The guide is
`docs/upgrade-v0.22-v0.28.md`, in the format of `docs/upgrade-v0.6-v0.22.md`.
No code changed.

## Sources

- CHANGELOG.md: BREAKING entries are v0.24.0 and v0.27.0 only.
- `git show <tag>:go.mod` for v0.22.5, v0.23.0, v0.24.0, v0.24.1 and v0.29.0:
  the module line changes at v0.24.0; no `replace` or `retract` anywhere.
- `git show v0.22.5:cmd/acphost/main.go`, `v0.26.0:cmd/acphost/app.go` and
  `session.go`: the v0.22 CLI had `run host start status stop wait`; the
  session commands came with v0.25.0 and went in v0.27.0, so a v0.22 user
  never had them. The sessions root at v0.26.0 was
  `$XDG_STATE_HOME/acphost/sessions`.
- Exported names of `pkg/` at v0.22.5 against HEAD (`git grep` of
  top-level exported declarations): additions only, no removal or
  signature change.
- `git diff v0.28.0 v0.29.0 -- pkg/acphost/host.go` for the v0.29 line.
- Notes 285, 306, 2026-11-13 and 2026-11-16, and the README headings.

## Checked

- The old path: in a throwaway module, `go get github.com/elee1766/acphost@v0.24.0`
  fails with "module declares its path as gfx-labs", and
  `github.com/gfx-labs/acphost@v0.23.0` fails the other way. The proxy lists
  every tag under both paths. The rewrite commands in the guide were run on
  a v0.22.5 module (imports, `MaybeChild` in `TestMain`) and it built and
  vetted on v0.28.0, then on v0.29.0. `urfave/cli` did not enter its
  `go.mod`. `go install .../cmd/acphost@v0.28.0` works at the new path and
  fails at the old.
- Snippets (TCP spec and `ConnectTCP`, `Reopen`) compile with `go vet` in a
  throwaway module against v0.28.0 and v0.29.0. They were not run.
- A host built on v0.22.5 and a client on v0.28.0: `Connect` got a welcome
  with `Protocol` 1, and the stop method ended the host.
- `acphost --help` and `acphost run --help` of v0.28.0 against the flags in
  v0.22.5's `main.go`: every v0.22 flag is there. `acpc` v0.2.0 `--help`
  for `new`, `prompt`, `rm`, `ls`.
- Every `docs/`, `pkg/` and `cmd/` path in the guide exists (the one at
  v0.26.0 is marked so), and each README heading it names is there.

## Not checked

- Whether `acpc` reads session dirs under the old `acphost/sessions` root.
  The guide says so.
- The TCP and v2 behavior at run time: taken from the code and notes, with
  their tests, not run again here.
- The v0.6-v0.22 guide says `go get github.com/gfx-labs/acphost@v0.22.0`
  and "the module path is unchanged", true only after v0.24. Out of scope
  here; filed as #314.

## README part (#313, items 3-4)

2026-11-16. README "Upgrading" links `docs/upgrade-v0.22-v0.28.md` and has
two more steps (module path v0.24, session commands to acpc v0.27). The
sentence on `Protocol` now says "1 in every release to date (v0.1.0 to
v0.29.0)": checked for v0.22 to v0.29 above, v0.1 to v0.22 in the older
guide.

"Release notes" had v0.22.1 as its newest entry (newest first). It now has
one entry per release v0.22.2 to v0.29.0, from CHANGELOG.md: dates and
issue numbers were compared with it by script (dates) and by `grep` (each
issue). v0.25.0, v0.25.1 and v0.26.0 have no issue number in CHANGELOG, so
none is given. No `docs/notes/NNN-release-*.md` was added: release-please
owns releases (#261). v0.29.1 is not released yet, so it has no entry.

Not done: the section still skips v0.11.0 to v0.21.0, which were never
listed. #313 asked for v0.22.2 onward.
