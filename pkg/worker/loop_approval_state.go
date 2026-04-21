package worker

import "github.com/insajin/autopus-adk/pkg/worker/a2a"

func (wl *WorkerLoop) storePendingApproval(params a2a.ApprovalRequestParams) {
	wl.approvalMu.Lock()
	defer wl.approvalMu.Unlock()
	if wl.pendingApprovals == nil {
		wl.pendingApprovals = make(map[string]a2a.ApprovalRequestParams)
	}
	wl.pendingApprovals[params.TaskID] = params
}

func (wl *WorkerLoop) pendingApproval(taskID string) (a2a.ApprovalRequestParams, bool) {
	wl.approvalMu.Lock()
	defer wl.approvalMu.Unlock()
	params, ok := wl.pendingApprovals[taskID]
	return params, ok
}

func (wl *WorkerLoop) clearPendingApproval(taskID string) {
	wl.approvalMu.Lock()
	defer wl.approvalMu.Unlock()
	delete(wl.pendingApprovals, taskID)
}
