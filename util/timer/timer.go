package timer

import (
	"time"
)

type Ticker interface {
	Stop()
}

type ticker struct {
	t    *time.Ticker
	stop chan struct{}
}

func NewTicker(d time.Duration, f func()) Ticker {
	t := &ticker{
		t:    time.NewTicker(d),
		stop: make(chan struct{}, 1),
	}

	go func() {
	EndFor:
		for {
			select {
			case <-t.t.C:
				f()
			case <-t.stop:
				break EndFor
			}
		}
		t.t.Stop()
	}()

	return t
}

func (t *ticker) Stop() {
	t.stop <- struct{}{}
}
