# Sequent

[![GoDoc](https://godoc.org/github.com/efritz/sequent?status.svg)](https://godoc.org/github.com/efritz/sequent)
[![Build Status](https://secure.travis-ci.org/efritz/sequent.png)](http://travis-ci.org/efritz/sequent)
[![Maintainability](https://api.codeclimate.com/v1/badges/14c3653af6e890607d74/maintainability)](https://codeclimate.com/github/efritz/sequent/maintainability)
[![Test Coverage](https://api.codeclimate.com/v1/badges/14c3653af6e890607d74/test_coverage)](https://codeclimate.com/github/efritz/sequent/test_coverage)

Go library for sequential background processing.

This library depends on the [watchdog](http://github.com/efritz/watchdog) and
[backoff](https://github.com/efritz/backoff) libraries, which will reinvoke methods
until success based on a given backoff strategy.

## Example

First, create an `Executor`. The alternate constructor, `NewExecutorWithBackoff`
creates an executor instance with a non-default backoff strategy. The default
strategy is an exponential strategy with some random jitter. Before scheduling
a task, the executor must be started, which will begin processing the work queue
in a goroutine.

```go
executor := NewExecutor()
executor.Start()
```

Tasks can then be scheduled via the `Schedule` method. A task should be a method
that returns true on success and false on failure. Each task is executed in the
order that it was scheduled; task *n* will not begin until task *n-1* has already
successfully finished.

```go
executor.Signal(func() bool {
    if ok := /* re-execute failed redis command */; ok {
        return true
    }

    fmt.Printf("Redis is still down.\n")
    return false
})
```

The user is also responsible for stopping the executor, which can be done by the
following two methods. Use the `Stop` method if the executor should abandon the
current task and ignore any queued tasks which have not yet been invoked. Use the
`Flush` method instead to wait for all queued tasks to finish processing.

```go
executor.Stop()  // immediate shutdown
executor.Flush() // graceful shutdown
```

## License

Copyright (c) 2017 Eric Fritz

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
