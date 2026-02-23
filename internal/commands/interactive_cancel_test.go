package commands

import (
	"context"
	"sync"
	"testing"
)

func TestCancelHolderTakeClears(t *testing.T) {
	var h cancelHolder
	called := false
	h.Set(func() { called = true })

	cancel := h.Take()
	if cancel == nil {
		t.Fatal("expected cancel func")
	}
	cancel()
	if !called {
		t.Fatal("expected cancel func to be called")
	}

	if cancel := h.Take(); cancel != nil {
		t.Fatal("expected cancel holder to be cleared after Take")
	}
}

func TestCancelHolderConcurrentAccess(t *testing.T) {
	var h cancelHolder
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			_ = ctx
			h.Set(cancel)
		}()
		go func() {
			defer wg.Done()
			if cancel := h.Take(); cancel != nil {
				cancel()
			}
		}()
	}

	wg.Wait()
	h.Clear()
}
