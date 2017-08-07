package sequent

import (
	"testing"

	"github.com/efritz/backoff"
	. "github.com/onsi/gomega"
)

type ExecutorSuite struct{}

var testBackoff = backoff.NewZeroBackoff()

func (s *ExecutorSuite) TestSchedule(t *testing.T) {
	var (
		c = make(chan int)
		r = make(chan int, 4)
		e = NewExecutor(WithBackoff(testBackoff))
	)

	defer close(r)

	e.Start()
	e.Schedule(func() bool { <-c; r <- 1; return true })
	e.Schedule(func() bool { r <- 2; return true })
	e.Schedule(func() bool { r <- 3; return true })
	e.Schedule(func() bool { r <- 4; return true })
	close(c)

	e.Flush()
	Expect(r).Should(Receive(Equal(1)))
	Expect(r).Should(Receive(Equal(2)))
	Expect(r).Should(Receive(Equal(3)))
	Expect(r).Should(Receive(Equal(4)))
}

func (s *ExecutorSuite) TestRetry(t *testing.T) {
	var (
		i = 0
		j = 0
		r = make(chan int)
		e = NewExecutor(WithBackoff(testBackoff))
	)

	e.Start()
	defer e.Stop()
	defer close(r)

	e.Schedule(func() bool { i++; r <- i; return i >= 20 })
	e.Schedule(func() bool { j++; r <- j; return j >= 10 })

	for k := 1; k <= 20; k++ {
		Eventually(r).Should(Receive(Equal(k)))
	}

	for k := 1; k <= 10; k++ {
		Eventually(r).Should(Receive(Equal(k)))
	}

	Eventually(r).ShouldNot(Receive())
}

func (s *ExecutorSuite) TestStop(t *testing.T) {
	var (
		r = make(chan int)
		e = NewExecutor(WithBackoff(testBackoff))
	)

	defer close(r)

	e.Start()
	e.Schedule(func() bool { r <- 1; return true })
	e.Schedule(func() bool { r <- 2; return true })
	e.Schedule(func() bool { r <- 3; return true })
	e.Schedule(func() bool { r <- 4; return true })
	e.Schedule(func() bool { r <- 5; return true })

	Eventually(r).Should(Receive(Equal(1)))
	Eventually(r).Should(Receive(Equal(2)))
	Eventually(r).Should(Receive(Equal(3)))
	e.Stop()
	Eventually(r).ShouldNot(Receive())
}
