package a2a

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TransitionListener is called when a task transitions between states.
type TransitionListener func(taskID string, from, to TaskStatus)

// validTransitions defines the allowed state transitions for the A2A task lifecycle.
var validTransitions = map[TaskStatus][]TaskStatus{
	StatusWorking:       {StatusCompleted, StatusFailed, StatusCanceled, StatusInputRequired},
	StatusInputRequired: {StatusWorking, StatusCanceled, StatusFailed},
	StatusCompleted:     {},
	StatusFailed:        {},
	StatusCanceled:      {},
}

// terminalStates is the set of states from which no further transitions are allowed.
var terminalStates = map[TaskStatus]bool{
	StatusCompleted: true,
	StatusFailed:    true,
	StatusCanceled:  true,
}

// TaskLifecycle manages state transitions for a single A2A task.
type TaskLifecycle struct {
	task      *Task
	mu        sync.Mutex
	listeners []TransitionListener
	createdAt time.Time
	updatedAt time.Time
}

// NewTaskLifecycle creates a lifecycle tracker for the given task.
func NewTaskLifecycle(task *Task) *TaskLifecycle {
	now := time.Now()
	return &TaskLifecycle{
		task:      task,
		createdAt: now,
		updatedAt: now,
	}
}

// Task returns the tracked task (snapshot under lock).
func (l *TaskLifecycle) Task() *Task {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.task
}

// Status returns the current task status.
func (l *TaskLifecycle) Status() TaskStatus {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.task.Status
}

// IsTerminal returns true if the task is in a terminal state.
func (l *TaskLifecycle) IsTerminal() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return terminalStates[l.task.Status]
}

// AddListener registers a transition listener.
func (l *TaskLifecycle) AddListener(fn TransitionListener) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.listeners = append(l.listeners, fn)
}

// Transition validates and executes a state transition.
// Returns an error if the transition is not allowed.
func (l *TaskLifecycle) Transition(to TaskStatus) error {
	l.mu.Lock()
	from := l.task.Status

	allowed, ok := validTransitions[from]
	if !ok {
		l.mu.Unlock()
		return fmt.Errorf("unknown current status: %s", from)
	}

	valid := false
	for _, s := range allowed {
		if s == to {
			valid = true
			break
		}
	}
	if !valid {
		l.mu.Unlock()
		return fmt.Errorf("invalid transition: %s -> %s", from, to)
	}

	l.task.Status = to
	l.updatedAt = time.Now()

	// Copy listeners under lock to invoke outside lock.
	listeners := make([]TransitionListener, len(l.listeners))
	copy(listeners, l.listeners)
	taskID := l.task.ID
	l.mu.Unlock()

	log.Printf("[a2a] task %s: %s -> %s", taskID, from, to)
	for _, fn := range listeners {
		fn(taskID, from, to)
	}
	return nil
}

// LifecycleManager tracks lifecycles for all active A2A tasks.
type LifecycleManager struct {
	tasks     map[string]*TaskLifecycle
	mu        sync.RWMutex
	listeners []TransitionListener
}

// NewLifecycleManager creates a new lifecycle manager.
func NewLifecycleManager() *LifecycleManager {
	return &LifecycleManager{
		tasks: make(map[string]*TaskLifecycle),
	}
}

// Track begins tracking the lifecycle of a task. Manager-level listeners
// are automatically added to the new lifecycle.
func (m *LifecycleManager) Track(task *Task) *TaskLifecycle {
	lc := NewTaskLifecycle(task)

	m.mu.Lock()
	defer m.mu.Unlock()

	lc.listeners = append(lc.listeners, m.listeners...)
	m.tasks[task.ID] = lc
	return lc
}

// Get retrieves the lifecycle for the given task ID.
func (m *LifecycleManager) Get(taskID string) (*TaskLifecycle, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	lc, ok := m.tasks[taskID]
	return lc, ok
}

// ActiveTasks returns all tasks that are not in a terminal state.
func (m *LifecycleManager) ActiveTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var active []*Task
	for _, lc := range m.tasks {
		lc.mu.Lock()
		status := lc.task.Status
		lc.mu.Unlock()
		if !terminalStates[status] {
			active = append(active, lc.task)
		}
	}
	return active
}

// Remove stops tracking a task lifecycle.
func (m *LifecycleManager) Remove(taskID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.tasks, taskID)
}

// AddListener registers a listener that will be added to all currently
// tracked and future task lifecycles.
func (m *LifecycleManager) AddListener(fn TransitionListener) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.listeners = append(m.listeners, fn)
	for _, lc := range m.tasks {
		lc.mu.Lock()
		lc.listeners = append(lc.listeners, fn)
		lc.mu.Unlock()
	}
}
