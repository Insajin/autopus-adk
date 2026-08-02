package cli

import (
	"errors"
	"os/exec"
)

type workflowContextLiveProcessTree struct {
	cmd     *exec.Cmd
	groupID int
	started bool
	closed  bool
}

func newWorkflowContextLiveProcessTree(cmd *exec.Cmd) *workflowContextLiveProcessTree {
	return &workflowContextLiveProcessTree{cmd: cmd}
}

func (tree *workflowContextLiveProcessTree) Start() error {
	if tree == nil || tree.cmd == nil || tree.started {
		return errors.New("process-tree-start-invalid")
	}
	if err := configureWorkflowContextLiveProcessGroup(tree.cmd); err != nil {
		return err
	}
	if err := tree.cmd.Start(); err != nil {
		return err
	}
	tree.started = true
	groupID, err := captureWorkflowContextLiveProcessGroup(tree.cmd)
	if err != nil {
		_ = terminateWorkflowContextLiveProcessGroup(tree.cmd.Process.Pid)
		_ = tree.cmd.Wait()
		tree.closed = true
		return err
	}
	tree.groupID = groupID
	return nil
}

func (tree *workflowContextLiveProcessTree) Close() error {
	if tree == nil || !tree.started || tree.closed {
		return nil
	}
	// Kill the captured process group before Wait reaps the leader. The leader's
	// PID therefore cannot be reused as an unrelated process while it identifies
	// this task-owned group.
	killErr := terminateWorkflowContextLiveProcessGroup(tree.groupID)
	waitErr := tree.cmd.Wait()
	tree.closed = true
	var exitErr *exec.ExitError
	if killErr != nil {
		return killErr
	}
	if waitErr != nil && !errors.As(waitErr, &exitErr) {
		return waitErr
	}
	return nil
}
