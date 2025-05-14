// Package volume provides functionality to listen for and retrieve system volume changes.
package volume

import (
	"sync"
	"time"
)

// Listener watches for system volume changes.
type Listener struct {
	mu      sync.Mutex
	lastVol int
	stopCh  chan struct{}
}

// NewListener creates and returns a new Listener for system volume changes.
func NewListener() *Listener {
	return &Listener{
		stopCh: make(chan struct{}),
	}
}

// GetCurrentVolume returns the current system output volume as a percentage (0-100).
func (l *Listener) GetCurrentVolume() (int, error) {
	return getSystemVolume()
}

// Listen returns a channel that emits the system volume percentage whenever it changes.
// The channel is closed when the listener is stopped.
func (l *Listener) Listen() (<-chan int, error) {
	ch := make(chan int)
	vol, err := l.GetCurrentVolume()
	if err != nil {
		return nil, err
	}

	l.lastVol = vol

	go func() {
		defer close(ch)
		for {
			select {
			case <-l.stopCh:
				return
			default:
				v, err := l.GetCurrentVolume()
				if err == nil {
					l.mu.Lock()
					if v != l.lastVol {
						l.lastVol = v
						ch <- v
					}
					l.mu.Unlock()
				}

				// Don't update too frequently
				// TODO: debounce?
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()
	return ch, nil
}
