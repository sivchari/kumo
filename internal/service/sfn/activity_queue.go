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
// for a task. claimed and ch are both guarded by the owning activityQueue's
// mutex, so enqueue and poll never race over which of them delivers (or
// fails to deliver) a given task; see activityQueue's own doc comment.
type activityQueueWaiter struct {
	ch      chan *activityTask
	claimed bool
}

// activityQueue implements one activity's task hand-off: a Task state
// enqueues a scheduled task, and GetActivityTask pollers dequeue one --
// each task delivered to exactly one poller, per AWS's GetActivityTask
// documentation. When a poller is already waiting, enqueue hands the task
// to it directly; otherwise the task sits in pending until a poller
// arrives.
//
// A poller that times out or whose context is done must still check
// whether it was claimed after all: enqueue may have already committed to
// delivering it a task (under waiters's lock) in the same instant the
// poller decided to give up. Coordinating this through the same mutex
// enqueue uses to pick a waiter is what makes delivery exactly-once even
// under that race, without ever silently dropping a scheduled task.
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

// giveUpOrClaimed resolves the race between a poller's own timeout/context
// cancellation and enqueue claiming the same waiter: if enqueue had already
// claimed w by the time this runs (under the same lock enqueue used), w.ch
// is guaranteed to have a task ready to read; otherwise w is removed from
// the queue so no later enqueue can claim it.
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
