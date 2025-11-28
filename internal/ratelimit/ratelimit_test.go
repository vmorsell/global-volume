package ratelimit

import (
	"testing"
	"time"
)

func TestNewRateLimiter(t *testing.T) {
	rl := NewRateLimiter(5, time.Second)
	if rl == nil {
		t.Fatal("NewRateLimiter returned nil")
	}
	if rl.limit != 5 {
		t.Errorf("expected limit 5, got %d", rl.limit)
	}
	if rl.window != time.Second {
		t.Errorf("expected window 1s, got %v", rl.window)
	}
}

func TestRateLimiter_Allow_FirstRequest(t *testing.T) {
	rl := NewRateLimiter(2, time.Second)
	key := "test-key"

	if !rl.Allow(key) {
		t.Error("first request should be allowed")
	}
}

func TestRateLimiter_Allow_WithinLimit(t *testing.T) {
	rl := NewRateLimiter(3, time.Second)
	key := "test-key"

	for i := 0; i < 3; i++ {
		if !rl.Allow(key) {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
}

func TestRateLimiter_Allow_ExceedsLimit(t *testing.T) {
	rl := NewRateLimiter(2, time.Second)
	key := "test-key"

	if !rl.Allow(key) {
		t.Error("first request should be allowed")
	}
	if !rl.Allow(key) {
		t.Error("second request should be allowed")
	}
	if rl.Allow(key) {
		t.Error("third request should be rate limited")
	}
}

func TestRateLimiter_Allow_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(2, time.Second)
	key1 := "key1"
	key2 := "key2"

	for i := 0; i < 2; i++ {
		if !rl.Allow(key1) {
			t.Errorf("key1 request %d should be allowed", i+1)
		}
		if !rl.Allow(key2) {
			t.Errorf("key2 request %d should be allowed", i+1)
		}
	}

	if rl.Allow(key1) {
		t.Error("key1 should be rate limited")
	}
	if rl.Allow(key2) {
		t.Error("key2 should be rate limited")
	}
}

func TestRateLimiter_Allow_WindowExpiration(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)
	key := "test-key"

	if !rl.Allow(key) {
		t.Error("first request should be allowed")
	}
	if !rl.Allow(key) {
		t.Error("second request should be allowed")
	}
	if rl.Allow(key) {
		t.Error("third request should be rate limited")
	}

	time.Sleep(150 * time.Millisecond)

	if !rl.Allow(key) {
		t.Error("request after window expiration should be allowed")
	}
}

func TestRateLimiter_Reset(t *testing.T) {
	rl := NewRateLimiter(2, time.Second)
	key := "test-key"

	if !rl.Allow(key) {
		t.Error("first request should be allowed")
	}
	if !rl.Allow(key) {
		t.Error("second request should be allowed")
	}
	if rl.Allow(key) {
		t.Error("third request should be rate limited")
	}

	rl.Reset()

	if !rl.Allow(key) {
		t.Error("request after reset should be allowed")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(2, 100*time.Millisecond)
	key1 := "key1"
	key2 := "key2"

	rl.Allow(key1)
	rl.Allow(key2)

	rl.mu.Lock()
	rl.cleanupTime = time.Now().Add(-1 * time.Minute)
	rl.mu.Unlock()

	time.Sleep(150 * time.Millisecond)
	rl.Allow(key1)

	rl.mu.Lock()
	if len(rl.requests) == 0 {
		t.Error("cleanup should not remove keys within cleanup window")
	}
	rl.mu.Unlock()
}

func TestRateLimiter_ConcurrentAccess(t *testing.T) {
	rl := NewRateLimiter(100, time.Second)
	key := "test-key"

	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 10; j++ {
				rl.Allow(key)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if rl.Allow(key) {
		t.Error("request after 100 concurrent requests should be rate limited")
	}
}
