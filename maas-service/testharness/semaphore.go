package testharness

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

type Semaphore struct {
	t       *testing.T
	awaitCh chan string
}

func NewSemaphore(t *testing.T) *Semaphore {
	return &Semaphore{t, make(chan string)}
}
func (s *Semaphore) Await(msg string, timeout time.Duration) {
	fmt.Printf("sem -> await message: %v\n", msg)
	select {
	case <-time.After(timeout):
		s.t.Fatalf("sem -> timeout waiting message: `%s'\n", msg)
	case e := <-s.awaitCh:
		fmt.Printf("sem -> received: `%v', expected: `%v'\n", e, msg)
		assert.Equal(s.t, msg, e)
		return
	}
}

func (s *Semaphore) Notify(msg string) {
	fmt.Printf("sem -> notify: %v\n", msg)
	s.awaitCh <- msg
}
