package orchestra

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type concurrencyProbeTerminal struct {
	mockTerminal
	active    atomic.Int32
	maxActive atomic.Int32
	nextID    atomic.Int32
	splitErr  error
}

func (m *concurrencyProbeTerminal) SplitPane(context.Context, terminal.Direction) (terminal.PaneID, error) {
	active := m.active.Add(1)
	defer m.active.Add(-1)
	for {
		maxActive := m.maxActive.Load()
		if active <= maxActive || m.maxActive.CompareAndSwap(maxActive, active) {
			break
		}
	}
	time.Sleep(15 * time.Millisecond)
	if m.splitErr != nil {
		return "", m.splitErr
	}
	return terminal.PaneID(fmt.Sprintf("surface:%d", m.nextID.Add(1))), nil
}

func TestSplitProviderPanes_ConcurrentOrchestras_SerializesTerminalSplits(t *testing.T) {
	isolateSurfaceTracker(t)
	term := &concurrencyProbeTerminal{mockTerminal: mockTerminal{name: "cmux"}}
	const workers = 8
	start := make(chan struct{})
	panesCh := make(chan []paneInfo, workers)
	errCh := make(chan error, workers)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			panes, _, err := splitProviderPanes(context.Background(), OrchestraConfig{
				Providers: []ProviderConfig{{Name: fmt.Sprintf("provider-%d", index)}},
				Terminal:  term,
			})
			if err != nil {
				errCh <- err
				return
			}
			panesCh <- panes
		}(i)
	}
	close(start)
	wg.Wait()
	close(errCh)
	close(panesCh)

	for err := range errCh {
		require.NoError(t, err)
	}
	for panes := range panesCh {
		cleanupPanes(term, panes)
	}
	assert.Equal(t, int32(1), term.maxActive.Load(),
		"terminal split operations share finite capacity and must be serialized")
}

func TestInteractivePaneBackend_ConcurrentProviders_SerializesTerminalSplits(t *testing.T) {
	term := &concurrencyProbeTerminal{
		mockTerminal: mockTerminal{name: "cmux"},
		splitErr:     errors.New("capacity busy"),
	}
	backend := NewInteractivePaneBackend(OrchestraConfig{
		Terminal:     term,
		FallbackMode: FallbackModeAbort,
	})
	const workers = 8
	start := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			_, _ = backend.Execute(context.Background(), ProviderRequest{
				Provider: fmt.Sprintf("provider-%d", index),
				Config:   ProviderConfig{Name: fmt.Sprintf("provider-%d", index)},
			})
		}(i)
	}
	close(start)
	wg.Wait()

	assert.Equal(t, int32(1), term.maxActive.Load(),
		"structured provider fan-out must share the terminal split serializer")
}
