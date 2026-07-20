package orchestra

import (
	"context"
	"errors"
	"fmt"
	"strings"
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

type paneInputOrderTerminal struct {
	mockTerminal
	eventsMu sync.Mutex
	events   []string
	sendErr  error
	enterErr error
}

func (m *paneInputOrderTerminal) SendLongText(_ context.Context, paneID terminal.PaneID, _ string) error {
	m.eventsMu.Lock()
	m.events = append(m.events, "send:"+string(paneID))
	err := m.sendErr
	m.eventsMu.Unlock()
	return err
}

func (m *paneInputOrderTerminal) SendCommand(_ context.Context, paneID terminal.PaneID, command string) error {
	m.eventsMu.Lock()
	m.events = append(m.events, "enter:"+string(paneID)+":"+command)
	err := m.enterErr
	m.eventsMu.Unlock()
	return err
}

func (m *paneInputOrderTerminal) snapshot() []string {
	m.eventsMu.Lock()
	defer m.eventsMu.Unlock()
	return append([]string(nil), m.events...)
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

func TestPaneInputCommit_ConcurrentCmuxPanes_DoesNotInterleavePasteAndEnter(t *testing.T) {
	term := &paneInputOrderTerminal{mockTerminal: mockTerminal{name: "cmux"}}
	start := make(chan struct{})
	errCh := make(chan error, 4)
	var wg sync.WaitGroup

	for _, paneID := range []terminal.PaneID{"surface:alpha", "surface:beta"} {
		paneID := paneID
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			sendErr, enterErr := sendPaneInputAndEnterSerialized(
				context.Background(), term, paneID, 10*time.Millisecond,
				func() error { return term.SendLongText(context.Background(), paneID, "launch") },
			)
			if sendErr != nil {
				errCh <- sendErr
			}
			if enterErr != nil {
				errCh <- enterErr
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}

	events := term.snapshot()
	require.Len(t, events, 4)
	firstPane := strings.TrimPrefix(events[0], "send:")
	secondPane := strings.TrimPrefix(events[2], "send:")
	assert.Equal(t, "enter:"+firstPane+":\n", events[1])
	assert.Equal(t, "enter:"+secondPane+":\n", events[3])
	assert.NotEqual(t, firstPane, secondPane)
}

func TestPaneInputCommit_CanceledWhileQueued_DoesNotSend(t *testing.T) {
	term := &paneInputOrderTerminal{mockTerminal: mockTerminal{name: "cmux"}}
	<-paneInputGate
	held := true
	t.Cleanup(func() {
		if held {
			paneInputGate <- struct{}{}
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	sendErr, enterErr := sendPaneInputAndEnterSerialized(ctx, term, "surface:queued", 0, func() error {
		return term.SendLongText(ctx, "surface:queued", "must-not-send")
	})

	require.ErrorIs(t, sendErr, context.DeadlineExceeded)
	assert.NoError(t, enterErr)
	assert.Empty(t, term.snapshot())

	paneInputGate <- struct{}{}
	held = false
	sendErr, enterErr = sendPaneInputAndEnterSerialized(context.Background(), term, "surface:next", 0, func() error {
		return term.SendLongText(context.Background(), "surface:next", "next")
	})
	require.NoError(t, sendErr)
	require.NoError(t, enterErr)
	assert.Equal(t, []string{"send:surface:next", "enter:surface:next:\n"}, term.snapshot(),
		"a canceled waiter must release the serializer for the next transaction")
}

func TestPaneInputCommit_FailuresReleaseCmuxGate(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*paneInputOrderTerminal, context.CancelFunc)
		wantSend  bool
		wantEnter bool
	}{
		{
			name: "send failure",
			configure: func(term *paneInputOrderTerminal, _ context.CancelFunc) {
				term.sendErr = errors.New("send failed")
			},
			wantSend: true,
		},
		{
			name: "delay cancellation",
			configure: func(_ *paneInputOrderTerminal, cancel context.CancelFunc) {
				go func() {
					time.Sleep(5 * time.Millisecond)
					cancel()
				}()
			},
			wantEnter: true,
		},
		{
			name: "enter failure",
			configure: func(term *paneInputOrderTerminal, _ context.CancelFunc) {
				term.enterErr = errors.New("enter failed")
			},
			wantEnter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := &paneInputOrderTerminal{mockTerminal: mockTerminal{name: "cmux"}}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			tt.configure(term, cancel)
			sendErr, enterErr := sendPaneInputAndEnterSerialized(ctx, term, "surface:failure", 20*time.Millisecond, func() error {
				return term.SendLongText(ctx, "surface:failure", "failure")
			})
			if tt.wantSend {
				require.Error(t, sendErr)
			}
			if tt.wantEnter {
				require.Error(t, enterErr)
			}

			term.sendErr = nil
			term.enterErr = nil
			nextSendErr, nextEnterErr := sendPaneInputAndEnterSerialized(context.Background(), term, "surface:next", 0, func() error {
				return term.SendLongText(context.Background(), "surface:next", "next")
			})
			require.NoError(t, nextSendErr)
			require.NoError(t, nextEnterErr)
		})
	}
}
