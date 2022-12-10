package xtimer

import (
	"time"
	"sync"
	"errors"
	"container/heap"
)

const _IDLE_DURATION = time.Second * 60

type _Task struct {
	tm time.Time
	fn func()
}

type _TaskHeap []_Task

func (h _TaskHeap) Len() int           { return len(h) }
func (h _TaskHeap) Less(i, j int) bool { return h[i].tm.Before(h[j].tm) }
func (h _TaskHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

// Push and Pop use pointer receivers because they modify the slice's length,
// not just its contents.
func (h *_TaskHeap) Push(x any) {
	*h = append(*h, x.(_Task))
}

func (h *_TaskHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0:n-1]
	return x
}

type Timer struct {
	tasks _TaskHeap
	ticker *time.Ticker
	resetChan chan struct{}
	running bool
	mutex sync.Mutex
}

// A new goroutine will created to wait for all running tasks
func New() *Timer {
	xt := &Timer{}
	xt.ticker = time.NewTicker(_IDLE_DURATION)
	xt.resetChan = make(chan struct{}, 1)
	xt.running = true
	go xt.waiting_routine()
	return xt
}

func (xt *Timer) waiting_routine() {
	for {
		select {
		case _, ok := <-xt.resetChan:
			if !ok {
				goto done
			}
		case <-xt.ticker.C:
		}

		var delta time.Duration
		for {
			var fn func()
			now := time.Now()
			xt.mutex.Lock()
			if len(xt.tasks) > 0 {
				t := &xt.tasks[0]
				if now.Before(t.tm) {
					delta = t.tm.Sub(now)
				} else {
					fn = t.fn
					heap.Pop(&xt.tasks)
				}
			} else {
				delta = _IDLE_DURATION
			}
			xt.mutex.Unlock()

			if fn == nil {
				break
			}

			fn()
		}
		xt.ticker.Reset(delta)
	}
done:
	xt.ticker.Stop()
	xt.ticker = nil
}

func (xt *Timer) Stop() {
	xt.mutex.Lock()
	if xt.running {
		xt.tasks = nil
		close(xt.resetChan)
	}
	xt.running = false
	xt.mutex.Unlock()
}

func (xt *Timer) Len() int {
	xt.mutex.Lock()
	n := len(xt.tasks)
	xt.mutex.Unlock()
	return n
}


var ErrTimerStopped = errors.New("xtimer.ErrTimerStopped")

// The only possible returned error is ErrTimerStopped
func (xt *Timer) AddTaskAt(fn func(), tm time.Time) error {
	if fn == nil {
		return nil
	}

	var err error
	xt.mutex.Lock()
	if xt.running {
		if len(xt.tasks) == 0 || tm.Before(xt.tasks[0].tm) {
			select {
			case xt.resetChan <- struct{}{}:
			default:
			}
		}
		heap.Push(&xt.tasks, _Task{tm, fn})
	} else {
		err = ErrTimerStopped
	}
	xt.mutex.Unlock()
	return err
}

func (xt *Timer) AddTaskAfter(fn func(), d time.Duration) error {
	tm := time.Now().Add(d)
	return xt.AddTaskAt(fn, tm)
}

