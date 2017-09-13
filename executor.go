package sequent

import (
	"context"
	"sync"
	"time"

	"github.com/efritz/backoff"
	"github.com/efritz/watchdog"
)

type (
	// Executor abstracts a sequence of tasks which are processed, in order, in a
	// background goroutine. New tasks can be scheduled in a non-blocking manner.
	Executor interface {
		// Start will begin processing scheduled tasks. This method must not block.
		Start()

		// Schedule will put a task at the end of the processing queue. This method
		// will block if Start has not been called, and must not be called after a
		// call to Stop or Flush.
		Schedule(task Task)

		// Stop immediately drops the current task and exit the processing loop. This
		// method does not attempt to interrupt the currently running task, but its
		// return value will be ignored by the calling function.
		Stop()

		// Flush blocks until the queue has been completely processed. This method is
		// the graceful version of Stop.
		Flush()
	}

	// Task is a function that returns true on success and false on failure.
	Task func() bool

	executor struct {
		backoff backoff.Backoff
		buffer  []Task
		mutex   *sync.Mutex
		tasks   chan Task
		halt    chan struct{}
		done    chan struct{}
		ready   chan struct{}
	}

	// ConfigFunc is a function used to initialize a new executor.
	ConfigFunc func(*executor)
)

// NewExecutor creates a new Executor.
func NewExecutor(configs ...ConfigFunc) Executor {
	backoff := backoff.NewExponentialBackoff(
		10*time.Millisecond,
		30*time.Second,
	)

	executor := &executor{
		backoff: backoff,
		buffer:  []Task{},
		mutex:   &sync.Mutex{},
		tasks:   make(chan Task),
		halt:    make(chan struct{}),
		done:    make(chan struct{}),
		ready:   make(chan struct{}, 1),
	}

	for _, config := range configs {
		config(executor)
	}

	return executor
}

// WithBackoff sets the backoff strategy to use (default is
// an exponential strategy with a maximum of 30 seconds).
func WithBackoff(backoff backoff.Backoff) ConfigFunc {
	return func(e *executor) {
		e.backoff = backoff
	}
}

func (e *executor) Start() {
	go e.queue()
	go e.process()
}

func (e *executor) Schedule(task Task) {
	e.tasks <- task
}

func (e *executor) Stop() {
	close(e.halt)
	close(e.tasks)
}

func (e *executor) Flush() {
	close(e.tasks)
	<-e.done
	close(e.halt)
}

func (e *executor) queue() {
	defer close(e.ready)

	// Put each task we get from calls to Schedule onto a
	// queue so that we don't block the producer.

	for task := range e.tasks {
		e.push(task)

		select {
		case e.ready <- struct{}{}:
		default:
		}
	}
}

func (e *executor) process() {
	// Close this channel once this goroutine exits so that
	// flush has something to synchronize on.
	defer close(e.done)

	// The outer loop condition blocks until a value is sent
	// on ready signifying a non-empty queue. The outer loop
	// halts once the halt channel is closed. This causes an
	// immediate exit of the process loop, regardless of the
	// state of the queue.

outer:
	for blockOnSignal(e.halt, e.ready) {
		// The inner loop will iterate while the halt channel
		// is still open. Once this channel is closed the loop
		// will hit the return below.

		for !isClosed(e.halt) {
			// While we have a task in the buffer attempt to
			// process it. If we have an empty queue, kick back
			// up to the outer loop and wait until the queue is
			// non-empty again (this prevents busy-waiting).

			task, ok := e.pop()
			if !ok {
				continue outer
			}

			e.call(task)
		}

		return
	}
}

func (e *executor) call(task Task) {
	// Create a context that is canceled when a value is received
	// on the halt channel (and, to clean up, when the watcher is
	// finished - this can be called twice without issue).

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		defer cancel()
		<-e.halt
	}()

	watchdog.BlockUntilSuccess(watchdog.RetryFunc(task), e.backoff, ctx)
}

func (e *executor) push(task Task) {
	e.mutex.Lock()
	e.buffer = append(e.buffer, task)
	e.mutex.Unlock()
}

func (e *executor) pop() (Task, bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if len(e.buffer) == 0 {
		return nil, false
	}

	var task Task
	task, e.buffer = e.buffer[0], e.buffer[1:]
	return task, true
}

func blockOnSignal(halt <-chan struct{}, ready <-chan struct{}) bool {
	select {
	case <-halt:
		return false

	case _, ok := <-ready:
		return ok
	}
}

func isClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true

	default:
		return false
	}
}
