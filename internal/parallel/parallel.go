// Package parallel runs independent tasks side by side.
package parallel

import (
	"runtime"
	"sync"
)

// RunAll runs independent tasks side by side, as many at once as there are CPUs: each hledger call is its own
// process. Every task runs; the error is the first failure in task order.
func RunAll(tasks ...func() error) error { return runLimited(runtime.NumCPU(), tasks) }

func runLimited(limit int, tasks []func() error) error {
	failures := make([]error, len(tasks))
	slots := make(chan struct{}, limit)
	var group sync.WaitGroup
	for i, task := range tasks {
		group.Go(func() {
			slots <- struct{}{}
			defer func() { <-slots }()
			failures[i] = task()
		})
	}
	group.Wait()
	for _, err := range failures {
		if err != nil {
			return err
		}
	}

	return nil
}
