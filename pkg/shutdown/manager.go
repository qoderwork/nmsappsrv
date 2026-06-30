package shutdown

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type Cleaner func(context.Context) error

type Manager struct {
	mu sync.RWMutex
	cleaners []Cleaner
	once sync.Once
	err error
}

func New() *Manager {
	return &Manager{}
}

// Register registers a cleanup function.
//
// Cleanup functions are executed in reverse registration order (LIFO),
// similar to defer.
func (m *Manager) Register(c Cleaner) {
	if c == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cleaners = append(m.cleaners, c)
}

// Wait waits until ctx is canceled, then performs shutdown.
func (m *Manager) Wait(ctx context.Context, timeout time.Duration) error {
	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return m.Shutdown(shutdownCtx)
}

// WaitSignal waits for SIGINT/SIGTERM by default.
func (m *Manager) WaitSignal(timeout time.Duration, sigs ...os.Signal) error {
	if len(sigs) == 0 {
		sigs = []os.Signal{
			syscall.SIGINT,
			syscall.SIGTERM,
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), sigs...)
	defer stop()

	return m.Wait(ctx, timeout)
}

// Shutdown executes cleanup functions once.
func (m *Manager) Shutdown(ctx context.Context) error {

	m.once.Do(func() {

		m.mu.RLock()
		cleaners := append([]Cleaner(nil), m.cleaners...)
		m.mu.RUnlock()

		var (
			wg    sync.WaitGroup
			errCh = make(chan error, len(cleaners))
		)

		// LIFO
		for i := len(cleaners) - 1; i >= 0; i-- {

			wg.Add(1)

			go func(c Cleaner) {
				defer wg.Done()

				if err := c(ctx); err != nil {
					errCh <- err
				}
			}(cleaners[i])
		}

		done := make(chan struct{})

		go func() {
			wg.Wait()
			close(done)
		}()

		select {

		case <-done:

		case <-ctx.Done():
			m.err = ctx.Err()
		}

		close(errCh)

		var errs []error

		if m.err != nil {
			errs = append(errs, m.err)
		}

		for err := range errCh {
			errs = append(errs, err)
		}

		if len(errs) > 0 {
			m.err = errors.Join(errs...)
		}

	})

	return m.err
}
