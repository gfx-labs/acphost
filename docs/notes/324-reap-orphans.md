# 324: reap orphans the host adopts

2026-11-20

The host sets `PR_SET_CHILD_SUBREAPER`, so a process the agent leaves
behind (a daemonized `git`, `ssh`, `tar`) reparents to it. Nothing waited
for them, and each exited one stayed a zombie holding a pid until the
host exited (464 zombies under 21 hosts on one machine).

`internal/children` reaps them on `SIGCHLD` and every 5 s. A blind
`Wait4(-1)` would also take the agent and the helpers that an `exec.Cmd`
waits for, and their `Wait` would fail with `ECHILD` and lose the exit
status. So every command the library starts goes through
`children.Start` or `Run`, which records the pid, and `Reap` waits only
for zombie children that are not recorded. A shared lock orders spawning
against reaping, so a child that exits at once is recorded before `Reap`
can see it. Recorded pids are dropped once `/proc/<pid>` is gone.

Zombie children are found in `/proc/self/task/*/children`, with a scan
of `/proc` when the kernel lacks it.

A command started with plain `exec.Cmd.Start` in a process that runs a
host (a test process running `Main`) can lose its exit status when it
is a zombie at the moment of a reap, so use `children.Start` for it.
