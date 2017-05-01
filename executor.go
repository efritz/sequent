package sequent

import (
	"sync"
	"time"

	"github.com/efritz/backoff"
	"github.com/efritz/watchdog"
)

type (
	Executor struct {
		backoff backoff.Backoff
		buffer  []Task
		mutex   *sync.Mutex
		tasks   chan Task
		halt    chan struct{}
		done    chan struct{}
		ready   chan struct{}
	}

	Task func() bool
)

// NewExecutor creates a new Executor with a default exponential backoff strategy.
func NewExecutor() *Executor {
	return NewExecutorWithBackoff(backoff.NewExponentialBackoff(
		2,
		0.25,
		10*time.Millisecond,
		30*time.Second,
	))
}

// NewExecutorWithbackoff creates a new Executor with the given backoff strategy.
func NewExecutorWithBackoff(backoff backoff.Backoff) *Executor {
	return &Executor{
		backoff: backoff,
		buffer:  []Task{},
		mutex:   &sync.Mutex{},
		tasks:   make(chan Task),
		halt:    make(chan struct{}),
		done:    make(chan struct{}),
		ready:   make(chan struct{}, 1),
	}
}

// Start will begin processing scheduled tasks. This method does not block.
func (e *Executor) Start() {
	go e.queue()
	go e.process()
}

// Schedule will put a task at the end of the processing queue. This method
// will block if Start has not been called, and must not be called after a
// call to Stop or Flush.
func (e *Executor) Schedule(task Task) {
	e.tasks <- task
}

// Stop immediately drops the current task and exit the processing loop. This
// method does not attempt to interrupt the currently running task, but its
// return value will be ignored by the calling function.
func (e *Executor) Stop() {
	close(e.halt)
	close(e.tasks)
}

// Flush blocks until the queue has been completely processed. This method is
// the graceful version of Stop.
func (e *Executor) Flush() {
	close(e.tasks)
	<-e.done
	close(e.halt)
}

//
// Background routines

func (e *Executor) queue() {
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

func (e *Executor) process() {
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

			watchdog.BlockUntilSuccessOrQuit(
				watchdog.RetryFunc(task),
				e.backoff,
				e.halt,
			)
		}

		return
	}
}

//
// Queueing primitives

func (e *Executor) push(task Task) {
	e.mutex.Lock()
	e.buffer = append(e.buffer, task)
	e.mutex.Unlock()
}

func (e *Executor) pop() (Task, bool) {
	e.mutex.Lock()
	defer e.mutex.Unlock()

	if len(e.buffer) == 0 {
		return nil, false
	}

	var task Task
	task, e.buffer = e.buffer[0], e.buffer[1:]
	return task, true
}

//
// Channel helpers

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
