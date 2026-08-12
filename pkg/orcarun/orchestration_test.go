package orcarun

import (
	"context"
	"strings"
	"testing"
	"time"
)

const runCreateResponse = `{"id":"req_1","ok":true,"error":null,"result":{"run":{` +
	`"id":"run_88012357eb61","objective":"ship the pipeline","home_database":"/tmp/db",` +
	`"coordinator_handle":"term_4d1","coordinator_pane_key":"pane_1","consumer_generation":1,` +
	`"legacy":0,"created_at":"2026-08-11T00:00:00Z","updated_at":"2026-08-11T00:00:00Z"},` +
	`"binding":{"consumerGeneration":1},"mutation":{"requestId":"mut_1","replayed":false}},` +
	`"_meta":{"runtimeId":"rt_1"}}`

const taskCreateResponse = `{"id":"req_2","ok":true,"error":null,"result":{"task":{` +
	`"id":"task_8c436a421b42","parent_id":null,"task_title":"plan","display_name":"plan",` +
	`"spec":"spec body","status":"ready","deps":"[]","result":null,"run_id":"run_88012357eb61",` +
	`"created_at":"2026-08-11T00:00:00Z"}},"_meta":{"runtimeId":"rt_1"}}`

const workerStartResponse = `{"id":"req_3","ok":true,"error":null,"result":{` +
	`"runId":"run_88012357eb61","taskId":"task_8c436a421b42","dispatchId":"ctx_532647ebc127",` +
	`"state":"ready","stage":"input_accepted","failedStage":null,"lastError":null,"setup":{},` +
	`"launch":{"requested":{"agent":"claude","model":"claude-opus-5","effort":"max"},` +
	`"effective":{"agent":"claude","model":"claude-opus-5","effort":"max"}},` +
	`"effects":[{"kind":"terminal","role":"agent","action":"created","id":"term_4d1","surface":"visible"}],` +
	`"residualResources":[],"mutation":{"requestId":"mut_3","replayed":false}},"_meta":{"runtimeId":"rt_1"}}`

const workerStartFailedResponse = `{"id":"req_4","ok":true,"error":null,"result":{` +
	`"runId":"run_88012357eb61","taskId":"task_8c436a421b42","dispatchId":"ctx_7a1b2c3d4e5f",` +
	`"state":"failed","stage":"agent_readiness","failedStage":"agent_readiness",` +
	`"lastError":"Agent startup blocked: codex-interactive-prompt",` +
	`"launch":{"requested":{"agent":"codex","model":"gpt-5.4-mini","effort":"high"},` +
	`"effective":{"agent":"codex","model":"gpt-5.4-mini","effort":"high"}},` +
	`"effects":[{"kind":"terminal","role":"agent","action":"created","id":"term_9f2","surface":"visible"}],` +
	`"residualResources":[{"kind":"terminal","role":"agent","action":"created","id":"term_9f2","surface":"visible"}],` +
	`"mutation":{"requestId":"mut_4","replayed":false}},"_meta":{"runtimeId":"rt_1"}}`

func TestCreateRunDecodesCoordinatorBinding(t *testing.T) {
	client, recorder := stubbedClient(runCreateResponse)

	run, err := client.CreateRun(context.Background(), "ship the pipeline")
	if err != nil {
		t.Fatalf("create run: %v", err)
	}
	if run.ID != "run_88012357eb61" || run.CoordinatorHandle != "term_4d1" {
		t.Fatalf("unexpected run %+v", run)
	}
	if run.Objective != "ship the pipeline" {
		t.Fatalf("unexpected objective %q", run.Objective)
	}
	if got := strings.Join(recorder.lastArgs(t), " "); got != "orchestration run-create --objective ship the pipeline --json" {
		t.Fatalf("unexpected argv: %s", got)
	}
}

// INV-101: the process plane must never hold a second task graph. No input may
// put a dependency flag on the wire, so the argv is checked exhaustively.
func TestCreateTaskNeverPassesDeps(t *testing.T) {
	cases := []struct{ name, runID, title, spec string }{
		{name: "with run", runID: "run_88012357eb61", title: "plan", spec: "spec body"},
		{name: "without run", title: "plan", spec: "spec body"},
		{name: "empty title and spec"},
		{name: "deps-shaped input", runID: "--deps", title: "--deps", spec: "--deps '[\"task_1\"]'"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			client, recorder := stubbedClient(taskCreateResponse)

			task, err := client.CreateTask(context.Background(), testCase.runID, testCase.title, testCase.spec)
			if err != nil {
				t.Fatalf("create task: %v", err)
			}
			if task.Deps != "[]" {
				t.Fatalf("expected an empty dependency set, got %q", task.Deps)
			}
			args := recorder.lastArgs(t)
			for index, arg := range args {
				if index > 0 && !strings.HasPrefix(args[index-1], "--") && arg == "--deps" {
					t.Fatalf("argv %v passed a dependency flag", args)
				}
				if strings.HasPrefix(arg, "--deps=") {
					t.Fatalf("argv %v passed a dependency flag", args)
				}
			}
		})
	}
}

func TestCreateTaskArgvShape(t *testing.T) {
	client, recorder := stubbedClient(taskCreateResponse, taskCreateResponse)
	ctx := context.Background()

	if _, err := client.CreateTask(ctx, "run_88012357eb61", "plan", "spec body"); err != nil {
		t.Fatalf("create task: %v", err)
	}
	got := strings.Join(recorder.lastArgs(t), " ")
	want := "orchestration task-create --spec spec body --task-title plan --run run_88012357eb61 --json"
	if got != want {
		t.Fatalf("unexpected argv: %s", got)
	}

	if _, err := client.CreateTask(ctx, "", "plan", "spec body"); err != nil {
		t.Fatalf("create task without run: %v", err)
	}
	if index := argvIndex(recorder.lastArgs(t), "--run"); index >= 0 {
		t.Fatalf("an empty run id must omit --run, got %v", recorder.lastArgs(t))
	}
}

func TestStartWorkerDecodesAdmission(t *testing.T) {
	client, recorder := stubbedClient(workerStartResponse)

	worker, err := client.StartWorker(context.Background(), StartRequest{
		RunID:    "run_88012357eb61",
		TaskID:   "task_8c436a421b42",
		Agent:    "claude",
		Model:    "claude-opus-5",
		Effort:   "max",
		Worktree: WorktreeCurrent,
		Timeout:  3 * time.Minute,
	})
	if err != nil {
		t.Fatalf("start worker: %v", err)
	}
	if worker.DispatchID != "ctx_532647ebc127" || worker.State != WorkerStateReady {
		t.Fatalf("unexpected worker %+v", worker)
	}
	if worker.Effective.Model != "claude-opus-5" || worker.Requested.Effort != "max" {
		t.Fatalf("launch was not decoded: %+v", worker)
	}
	if len(worker.Residual) != 0 {
		t.Fatalf("a clean launch leaves no residual, got %+v", worker.Residual)
	}

	args := recorder.lastArgs(t)
	want := "orchestration worker-start --task task_8c436a421b42 --worktree current --agent claude " +
		"--model claude-opus-5 --effort max --run run_88012357eb61 --timeout-ms 180000 --json"
	if got := strings.Join(args, " "); got != want {
		t.Fatalf("unexpected argv: %s", got)
	}
}

// A readiness failure creates a terminal that no dispatch owns. Losing the
// residual set loses the only handle on that terminal.
func TestStartWorkerKeepsResidualResources(t *testing.T) {
	client, _ := stubbedClient(workerStartFailedResponse)

	worker, err := client.StartWorker(context.Background(), StartRequest{
		TaskID:   "task_8c436a421b42",
		Agent:    "codex",
		Worktree: WorktreeCurrent,
	})
	if err != nil {
		t.Fatalf("start worker: %v", err)
	}
	if worker.State != "failed" || worker.Stage != "agent_readiness" {
		t.Fatalf("unexpected failure state %+v", worker)
	}
	if worker.LastError != "Agent startup blocked: codex-interactive-prompt" {
		t.Fatalf("unexpected last error %q", worker.LastError)
	}
	if len(worker.Residual) != 1 {
		t.Fatalf("expected one residual resource, got %+v", worker.Residual)
	}
	residual := worker.Residual[0]
	if residual.ID != "term_9f2" || residual.Kind != "terminal" || residual.Role != "agent" {
		t.Fatalf("unexpected residual %+v", residual)
	}
	if residual.Action != "created" {
		t.Fatalf("unexpected residual action %q", residual.Action)
	}
}

// An absent model or effort must drop its flag. Passing a blank value is not
// the same request, and --effort without --model is rejected outright.
func TestStartWorkerOmitsEmptyLaunchFlags(t *testing.T) {
	client, recorder := stubbedClient(workerStartResponse)

	_, err := client.StartWorker(context.Background(), StartRequest{
		TaskID:   "task_8c436a421b42",
		Agent:    "claude",
		Worktree: WorktreeCurrent,
	})
	if err != nil {
		t.Fatalf("start worker: %v", err)
	}
	args := recorder.lastArgs(t)
	for _, flag := range []string{"--model", "--effort", "--run", "--timeout-ms"} {
		if argvIndex(args, flag) >= 0 {
			t.Fatalf("argv %v must omit %s when it is unset", args, flag)
		}
	}
	for _, arg := range args {
		if arg == "" {
			t.Fatalf("argv %v carries a blank value", args)
		}
	}
	if got := argvValue(t, args, "--agent"); got != "claude" {
		t.Fatalf("unexpected agent %q", got)
	}
}
