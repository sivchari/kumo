package sfn

import (
	"context"
	"sync"
	"time"
)

// activityTask is a single unit of work scheduled for an activity by a Task
// state, waiting to be handed to a GetActivityTask poller.
type activityTask struct {
	token   string
	input   string
	pending *pendingCallback
}

// activityQueueWaiter is a single GetActivityTask long poll blocked waiting
// for a task. claimed and ch are guarded by activityQueue's mutex so
// enqueue and poll never race over delivery.
type activityQueueWaiter struct {
	ch      chan *activityTask
	claimed bool
}

// activityQueue hands off one activity's scheduled tasks to GetActivityTask
// pollers, each task delivered to exactly one poller (per AWS's
// GetActivityTask docs). A poller that gives up on timeout must recheck
// under the same mutex whether enqueue already claimed it in that instant,
// so delivery stays exactly-once without ever dropping a task.
type activityQueue struct {
	mu      sync.Mutex
	pending []*activityTask
	waiters []*activityQueueWaiter
}

// newActivityQueue creates an empty activityQueue.
func newActivityQueue() *activityQueue {
	return &activityQueue{}
}

// enqueue schedules task, handing it directly to the longest-waiting poller
// if one is available, or appending it to pending otherwise.
func (q *activityQueue) enqueue(task *activityTask) {
	q.mu.Lock()

	if len(q.waiters) > 0 {
		w := q.waiters[0]
		q.waiters = q.waiters[1:]
		w.claimed = true

		q.mu.Unlock()

		w.ch <- task

		return
	}

	q.pending = append(q.pending, task)

	q.mu.Unlock()
}

// poll waits up to timeout (or until ctx is done, if sooner) for a task,
// returning nil if none arrived in time.
func (q *activityQueue) poll(ctx context.Context, timeout time.Duration) *activityTask {
	q.mu.Lock()

	if len(q.pending) > 0 {
		task := q.pending[0]
		q.pending = q.pending[1:]

		q.mu.Unlock()

		return task
	}

	w := &activityQueueWaiter{ch: make(chan *activityTask, 1)}
	q.waiters = append(q.waiters, w)

	q.mu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case task := <-w.ch:
		return task
	case <-timer.C:
	case <-ctx.Done():
	}

	return q.giveUpOrClaimed(w)
}

// giveUpOrClaimed resolves the race between a poller giving up and enqueue
// claiming the same waiter: if already claimed, w.ch is guaranteed to have
// a task ready; otherwise w is removed so no later enqueue can claim it.
func (q *activityQueue) giveUpOrClaimed(w *activityQueueWaiter) *activityTask {
	q.mu.Lock()

	if w.claimed {
		q.mu.Unlock()

		return <-w.ch
	}

	for i, waiter := range q.waiters {
		if waiter == w {
			q.waiters = append(q.waiters[:i], q.waiters[i+1:]...)

			break
		}
	}

	q.mu.Unlock()

	return nil
}
