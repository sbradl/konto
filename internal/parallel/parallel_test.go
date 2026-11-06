package parallel

import (
	"errors"
	"sync/atomic"
	"testing"
)

func TestRunAllRunsEveryTaskAndReturnsTheFirstFailureInTaskOrder(t *testing.T) {
	second, third := errors.New("second"), errors.New("third")
	var ran atomic.Int32
	task := func(err error) func() error {
		return func() error {
			ran.Add(1)

			return err
		}
	}

	err := RunAll(task(nil), task(second), task(third))

	if !errors.Is(err, second) || errors.Is(err, third) || ran.Load() != 3 {
		t.Errorf("RunAll = %v after %d tasks; want the second task's failure after all three", err, ran.Load())
	}

	if err := RunAll(task(nil), task(nil)); err != nil {
		t.Errorf("RunAll = %v, want nil when every task succeeds", err)
	}
}

func TestRunAllRunsNoMoreThanTheLimitAtOnce(t *testing.T) {
	var running, most atomic.Int32
	task := func() error {
		now := running.Add(1)
		defer running.Add(-1)
		for {
			if seen := most.Load(); now <= seen || most.CompareAndSwap(seen, now) {
				break
			}
		}

		return nil
	}

	_ = runLimited(1, []func() error{task, task, task, task})

	if most.Load() != 1 {
		t.Errorf("%d tasks ran at once, want at most 1", most.Load())
	}
}
