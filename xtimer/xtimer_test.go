package xtimer

import (
	"testing"
	"fmt"
	"time"
)

func task(i int) func() {
	return func() {
		fmt.Println("task", i)
	}
}

func TestXTimer(t *testing.T) {
	tm := New()
	tm.AddTaskAfter(task(1), time.Second*5)
	tm.AddTaskAfter(task(2), time.Second*4)
	tm.AddTaskAfter(task(3), time.Second*3)
	tm.AddTaskAfter(task(4), time.Second*2)
	tm.AddTaskAfter(task(5), time.Second*1)

	for tm.Len() > 2 {
		time.Sleep(time.Second)
	}
	tm.Stop()
}

