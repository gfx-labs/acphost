# 320: TestSystemdOOM flaked, the holder was OOM killed too

2026-11-16

Master CI at 91b53430 (docs only) and PR #315 failed TestSystemdOOM:
exit Signal killed, OOMKill false, cgroup stats all 0. Under CPU load
(`go test -race -count=80`, four busy loops) it failed about 3 runs in 100.

A debug print of the scope's `cgroup.procs` and `memory.events` in the
failing runs showed `oom_kill 2` and no processes: the scope's holder
(the re-exec'd pin helper, in the scope proper) counts against the
scope's memory.max, and the kernel killed it along with the command.
systemd collected the empty scope before `proc.Wait` read the stats.
Real agents over their limit could lose the oom report this way.

Fix: with the systemd driver the command's oom_score_adj is raised to
500 right after start, so it is chosen before the holder. An
unprivileged process may raise it, and children inherit it. A child
forked in the first moments keeps 0; the command's main process is the
one that holds the memory. With the fix 99 runs under the same load
passed, none failed.

Not done: moving the limits to the leaf, which would keep the holder out
of them entirely but changes what Isolation reports as applied.
