package kcp

import (
	"container/heap"
	"runtime"
	"sync"
	"time"
)

var _SystemTimedSched *TimedSched = NewTimedSched(runtime.NumCPU())

type timedFunc struct {
	execute func()
	ts      time.Time
}

type timedFuncHeap []timedFunc

func (h timedFuncHeap) Len() int            { return len(h) }
func (h timedFuncHeap) Less(i, j int) bool  { return h[i].ts.Before(h[j].ts) }
func (h timedFuncHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *timedFuncHeap) Push(x interface{}) { *h = append(*h, x.(timedFunc)) }
func (h *timedFuncHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1].execute = nil
	*h = old[0 : n-1]

	return x
}

type TimedSched struct {
	prependTasks    []timedFunc
	prependLock     sync.Mutex
	chPrependNotify chan struct{}

	chTask chan timedFunc

	dieOnce sync.Once
	die     chan struct{}
}

func NewTimedSched(parallel int) *TimedSched {
	ts := new(TimedSched)
	ts.chTask = make(chan timedFunc)
	ts.die = make(chan struct{})
	ts.chPrependNotify = make(chan struct{}, 1)

	for i := 0; i < parallel; i++ {
		go ts.sched()
	}

	go ts.prepend()

	return ts
}

func (ts *TimedSched) sched() {
	var tasks timedFuncHeap

	timer := time.NewTimer(0)
	drained := false

	for {
		select {
		case task := <-ts.chTask:
			now := time.Now()
			if now.After(task.ts) {
				task.execute()
			} else {
				heap.Push(&tasks, task)
				stopped := timer.Stop()
				if !stopped && !drained {
					<-timer.C
				}
				timer.Reset(tasks[0].ts.Sub(now))
				drained = false
			}

		case now := <-timer.C:
			drained = true

			for tasks.Len() > 0 {
				if now.After(tasks[0].ts) {
					heap.Pop(&tasks).(timedFunc).execute()
				} else {
					timer.Reset(tasks[0].ts.Sub(now))
					drained = false

					break
				}
			}

		case <-ts.die:
			return
		}
	}
}

func (ts *TimedSched) prepend() {
	var tasks []timedFunc

	for {
		select {
		case <-ts.chPrependNotify:
			ts.prependLock.Lock()

			if cap(tasks) < cap(ts.prependTasks) {
				tasks = make([]timedFunc, 0, cap(ts.prependTasks))
			}

			tasks = tasks[:len(ts.prependTasks)]
			copy(tasks, ts.prependTasks)

			for k := range ts.prependTasks {
				ts.prependTasks[k].execute = nil
			}

			ts.prependTasks = ts.prependTasks[:0]
			ts.prependLock.Unlock()

			for k := range tasks {
				select {
				case ts.chTask <- tasks[k]:
					tasks[k].execute = nil
				case <-ts.die:
					return
				}
			}

			tasks = tasks[:0]

		case <-ts.die:
			return
		}
	}
}

func (ts *TimedSched) Put(f func(), deadline time.Time) {
	ts.prependLock.Lock()
	ts.prependTasks = append(ts.prependTasks, timedFunc{f, deadline})
	ts.prependLock.Unlock()

	select {
	case ts.chPrependNotify <- struct{}{}:
	default:
	}
}

func (ts *TimedSched) Close() { ts.dieOnce.Do(func() { close(ts.die) }) }
