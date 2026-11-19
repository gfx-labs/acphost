package children

import (
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// reapEvery backs up SIGCHLD, which Go delivers coalesced.
const reapEvery = 5 * time.Second

// StartReaper reaps orphans on SIGCHLD and every few seconds until the
// returned function is called. The caller makes the process a subreaper
// first; without that there are no orphans to reap.
func StartReaper(log *slog.Logger) (stop func()) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGCHLD)
	done := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		tick := time.NewTicker(reapEvery)
		defer tick.Stop()
		for {
			select {
			case <-done:
				return
			case <-sigs:
			case <-tick.C:
			}
			if n := Reap(); n > 0 {
				log.Debug("reaped orphans", "count", n)
			}
		}
	}()
	return func() {
		signal.Stop(sigs)
		close(done)
		wg.Wait()
	}
}
