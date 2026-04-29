package state

import (
	"sync"
	"testing"
)

func TestStationCache_ConcurrentReadWrite(t *testing.T) {
	c := NewStationCache()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			c.UpdateState("5", "VIN", "fw")
			_, _ = c.GetCurrentState("5")
		}(i)
	}
	wg.Wait()
}

func TestStationCache_Isolation(t *testing.T) {
	c := NewStationCache()
	c.UpdateState("5", "A", "1")
	c.UpdateState("6", "B", "2")
	v, f := c.GetCurrentState("5")
	if v != "A" || f != "1" {
		t.Fatalf("station 5: got %q %q", v, f)
	}
	v, f = c.GetCurrentState("6")
	if v != "B" || f != "2" {
		t.Fatalf("station 6: got %q %q", v, f)
	}
}
