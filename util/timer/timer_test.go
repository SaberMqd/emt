package timer

import (
	"fmt"
	"testing"
	"time"
)

func Test_Error(t *testing.T) {
	fmt.Println("test error")

	t1 := NewTicker(3*time.Second, func() {
		fmt.Println("tick go")
	})

	time.Sleep(10 * time.Second)
	t1.Stop()
}
