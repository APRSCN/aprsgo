// Package wait provides small concurrency helpers shared across subsystems.
package wait

import "time"

// SleepOrStop waits for d to elapse or until stop is closed, whichever happens
// first. It returns true if the full duration elapsed, or false if stop fired
// (i.e. a stop/shutdown was requested) before then. The timer is always
// released, so it is safe to call in a tight reconnect loop.
func SleepOrStop(stop <-chan struct{}, d time.Duration) bool {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-stop:
		return false
	case <-t.C:
		return true
	}
}
