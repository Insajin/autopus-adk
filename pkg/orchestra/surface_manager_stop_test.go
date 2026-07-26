package orchestra

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/require"
)

type blockingSurfaceSignalMock struct {
	surfaceSignalMock
	healthEntered chan struct{}
	releaseHealth chan struct{}
	enterOnce     sync.Once
}

func (m *blockingSurfaceSignalMock) SurfaceHealth(_ context.Context, paneID terminal.PaneID) (terminal.SurfaceStatus, error) {
	m.enterOnce.Do(func() {
		close(m.healthEntered)
	})
	<-m.releaseHealth
	return terminal.SurfaceStatus{
		Valid:      true,
		SurfaceRef: string(paneID),
		InWindow:   true,
	}, nil
}

func TestSurfaceManager_StopWaitsForMonitorAndConcurrentCallers(t *testing.T) {
	mock := &blockingSurfaceSignalMock{
		healthEntered: make(chan struct{}),
		releaseHealth: make(chan struct{}),
	}
	mock.name = "cmux"

	sm := NewSurfaceManager(mock)
	sm.interval = time.Millisecond
	var releaseOnce sync.Once
	releaseHealth := func() {
		releaseOnce.Do(func() {
			close(mock.releaseHealth)
		})
	}
	t.Cleanup(func() {
		releaseHealth()
		sm.Stop()
	})

	panes := []paneInfo{
		{paneID: "pane-1", provider: ProviderConfig{Name: "claude"}},
	}
	sm.Start(context.Background(), panes)

	select {
	case <-mock.healthEntered:
	case <-time.After(2 * time.Second):
		t.Fatal("monitor did not enter SurfaceHealth")
	}

	const stopCallers = 4
	startStop := make(chan struct{})
	stopReturned := make(chan struct{}, stopCallers)
	for range stopCallers {
		go func() {
			<-startStop
			sm.Stop()
			stopReturned <- struct{}{}
		}()
	}
	close(startStop)

	select {
	case <-stopReturned:
		releaseHealth()
		t.Fatal("Stop returned while monitor was still running")
	case <-time.After(100 * time.Millisecond):
	}

	releaseHealth()
	for range stopCallers {
		select {
		case <-stopReturned:
		case <-time.After(2 * time.Second):
			t.Fatal("Stop did not return after monitor exited")
		}
	}

	// Stop is the ownership handoff after which callers may mutate shared panes.
	panes[0].paneID = ""
	require.NotPanics(t, sm.Stop, "repeated Stop should remain safe")
}
