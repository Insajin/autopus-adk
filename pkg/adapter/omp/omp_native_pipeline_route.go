package omp

import "github.com/insajin/autopus-adk/pkg/adapter"

const (
	ompNativePipelineRouteTarget = ".omp/extensions/autopus-pipeline.ts"
	// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: embedded native /auto is the external OMP-to-pipeline command boundary.
	// @AX:REASON [AUTO]: Argument admission, shell-free spawning, managed-inner recursion denial, and generated identity must stay aligned.
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: arguments, captured output, and notices are bounded to 512, 4096, and 320 characters.
	ompNativePipelineRouteSource = `import { spawn } from "node:child_process";

type CommandContext = {
  cwd: string;
  ui: {
    notify(message: string, level?: "info" | "warning" | "error"): void;
  };
};

type CommandAPI = {
  registerCommand(
    name: string,
    command: {
      description: string;
      handler(args: string, context: CommandContext): Promise<void>;
    },
  ): void;
};

type ParsedRoute =
  | { ok: true; specID: string; forwardedFlags: string[] }
  | { ok: false; message: string };

type RunResult = {
  code: number;
  stdout: string;
  stderr: string;
};

const SPEC_ID_PATTERN = /^SPEC-[A-Z0-9]+(?:-[A-Z0-9]+)+$/;
const MAX_ARGUMENT_LENGTH = 512;
const MAX_CAPTURE_LENGTH = 4096;
const MAX_NOTICE_LENGTH = 320;
const USAGE = "Usage: /auto go SPEC-ID [--continue] [--dry-run] [--strategy sequential|parallel] [--auto] [--loop] [--multi] [--quality balanced|ultra] [--effort low|medium|high|xhigh|max|ultra]";

const BOOLEAN_FLAGS = new Set([
  "--continue",
  "--dry-run",
  "--auto",
  "--loop",
  "--multi",
]);

const VALUE_FLAGS = new Map<string, ReadonlySet<string>>([
  ["--strategy", new Set(["sequential", "parallel"])],
  ["--quality", new Set(["balanced", "ultra"])],
  ["--effort", new Set(["low", "medium", "high", "xhigh", "max", "ultra"])],
]);

function parseRoute(args: string): ParsedRoute {
  const trimmed = args.trim();
  if (trimmed.length === 0 || trimmed.length > MAX_ARGUMENT_LENGTH) {
    return { ok: false, message: USAGE };
  }
  const tokens = trimmed.split(/\s+/);
  if (tokens.length < 2 || tokens[0] !== "go" || !SPEC_ID_PATTERN.test(tokens[1])) {
    return { ok: false, message: USAGE };
  }

  const forwardedFlags: string[] = [];
  const seen = new Set<string>();
  for (let index = 2; index < tokens.length; index++) {
    const flag = tokens[index];
    if (seen.has(flag)) {
      return { ok: false, message: "Duplicate or unsupported /auto argument. " + USAGE };
    }
    if (BOOLEAN_FLAGS.has(flag)) {
      seen.add(flag);
      forwardedFlags.push(flag);
      continue;
    }
    const allowedValues = VALUE_FLAGS.get(flag);
    const value = tokens[index + 1];
    if (allowedValues === undefined || value === undefined || !allowedValues.has(value)) {
      return { ok: false, message: "Unsupported /auto argument. " + USAGE };
    }
    seen.add(flag);
    forwardedFlags.push(flag, value);
    index++;
  }
  return { ok: true, specID: tokens[1], forwardedFlags };
}

function appendCaptured(current: string, chunk: string): string {
  const joined = current + chunk;
  return joined.length <= MAX_CAPTURE_LENGTH ? joined : joined.slice(-MAX_CAPTURE_LENGTH);
}

function runAuto(argv: string[], cwd: string): Promise<RunResult> {
  return new Promise((resolve) => {
    let stdout = "";
    let stderr = "";
    let settled = false;
    const child = spawn("auto", argv, {
      cwd,
      shell: false,
      stdio: ["ignore", "pipe", "pipe"],
    });
    child.stdout?.setEncoding("utf8");
    child.stderr?.setEncoding("utf8");
    child.stdout?.on("data", (chunk: string) => {
      stdout = appendCaptured(stdout, chunk);
    });
    child.stderr?.on("data", (chunk: string) => {
      stderr = appendCaptured(stderr, chunk);
    });
    const finish = (code: number): void => {
      if (settled) {
        return;
      }
      settled = true;
      resolve({ code, stdout, stderr });
    };
    child.once("error", (error) => {
      stderr = appendCaptured(stderr, error.message);
      finish(127);
    });
    child.once("close", (code) => finish(code ?? 1));
  });
}

function conciseOutput(result: RunResult): string {
  const output = (result.stderr.trim() || result.stdout.trim()).replace(/\s+/g, " ");
  if (output.length <= MAX_NOTICE_LENGTH) {
    return output;
  }
  return output.slice(0, MAX_NOTICE_LENGTH - 3) + "...";
}

export default function autopusPipelineRoute(pi: CommandAPI): void {
  if (process.env.AUTOPUS_OMP_MANAGED_INNER === "1") {
    return;
  }
  pi.registerCommand("auto", {
    description: "Run an Autopus SPEC through the native OMP pipeline",
    handler: async (args, context) => {
      const parsed = parseRoute(args);
      if (!parsed.ok) {
        context.ui.notify(parsed.message, "error");
        return;
      }
      const { specID, forwardedFlags } = parsed;
      const childArgv = ["pipeline", "run", specID, "--platform", "omp"];
      childArgv.push(...forwardedFlags);
      context.ui.notify("Starting Autopus pipeline for " + specID + ".", "info");
      const result = await runAuto(childArgv, context.cwd);
      const detail = conciseOutput(result);
      if (result.code !== 0) {
        const suffix = detail === "" ? "" : " " + detail;
        context.ui.notify("Autopus pipeline failed (exit " + result.code + ")." + suffix, "error");
        return;
      }
      const suffix = detail === "" ? "" : " " + detail;
      context.ui.notify("Autopus pipeline completed for " + specID + "." + suffix, "info");
    },
  });
}
`
)

// OMPNativePipelineRouteSourceIdentity describes the generated native command owner.
type OMPNativePipelineRouteSourceIdentity struct {
	TargetPath string
	SHA256     string
	Size       int64
}

// ExpectedOMPNativePipelineRouteSourceIdentity returns the immutable embedded route identity.
func ExpectedOMPNativePipelineRouteSourceIdentity() OMPNativePipelineRouteSourceIdentity {
	return OMPNativePipelineRouteSourceIdentity{
		TargetPath: ompNativePipelineRouteTarget,
		SHA256:     adapter.Checksum(ompNativePipelineRouteSource),
		Size:       int64(len(ompNativePipelineRouteSource)),
	}
}

func prepareOMPNativePipelineRouteMapping() adapter.FileMapping {
	return adapter.FileMapping{
		SourceTemplate:  "embedded:omp-native-pipeline-route-v1",
		TargetPath:      ompNativePipelineRouteTarget,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        adapter.Checksum(ompNativePipelineRouteSource),
		Content:         []byte(ompNativePipelineRouteSource),
	}
}
