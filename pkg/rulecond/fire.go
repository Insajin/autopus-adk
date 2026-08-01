package rulecond

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
)

// maxStdinBytes bounds the untrusted hook payload read.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-CONDRULE-001: 1 MiB magic constant bounds the untrusted hook stdin payload; the LimitReader turns an oversized payload into a decode failure, which fails open.
const maxStdinBytes = 1 << 20

// hookPayload is the subset of the hook stdin payload the dispatcher reads.
type hookPayload struct {
	HookEventName string         `json:"hook_event_name"`
	ToolName      string         `json:"tool_name"`
	ToolInput     map[string]any `json:"tool_input"`
}

// Fire runs one dispatcher invocation against the manifest under root.
//
// The dispatcher is advisory and always exits zero. Benign absence is
// fail-open: a missing or malformed manifest, an uncompilable stored regex, an
// unparseable payload, a tool with no condition subject, no condition match, or
// a body file that is simply gone produces no output and no diagnostic. A
// containment or size violation is fail-closed: it suppresses only the
// offending rule, reports the rule name and reason code on stderr, and leaves
// every other matched and contained rule injected (REQ-CONDRULE-FIRE-09).
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: advisory dispatcher entry point — the generated PreToolUse hook and internal/cli/rules.go both depend on the exit-zero contract and the stdout JSON shape.
// @AX:REASON: Every bare `return nil` below discards its error on purpose; propagating one would turn a dispatcher fault into a blocked tool call for the user.
func Fire(root, event string, stdin io.Reader, stdout, stderr io.Writer) error {
	// The manifest location is resolved under the trusted-root contract before
	// it is opened. A repository that ships `.claude` as a symlink relocates the
	// manifest itself, so this refusal is fail-closed for the whole run rather
	// than the fail-open a missing manifest gets.
	manifestPath, err := ResolveManifestPath(root)
	if err != nil {
		reportViolation(stderr, manifestDiagnosticName, err)
		return nil
	}
	manifest, rejected, err := loadManifest(manifestPath)
	if err != nil {
		return nil
	}
	// A rule dropped for a non-conforming Name is a fail-closed suppression, so
	// it is reported like a containment violation: the sanitized name and the
	// reason code, nothing else. It is reported here rather than at match time
	// because the rule is removed before matching, and a manifest that ships a
	// forged name is evidence regardless of which tool call observed it.
	reportRejectedNames(rejected, stderr)

	payload, err := decodePayload(stdin)
	if err != nil {
		return nil
	}
	if event == "" {
		event = payload.HookEventName
	}
	if event == "" {
		event = EventPreToolUse
	}

	subject, ok := ConditionSubject(payload.ToolName, payload.ToolInput)
	if !ok {
		return nil
	}
	matched, err := matchRules(manifest, event, payload.ToolName, subject)
	if err != nil || len(matched) == 0 {
		return nil
	}

	entries := readBodies(root, matched, stderr)
	if len(entries) == 0 {
		return nil
	}
	return writeOutput(stdout, event, assembleContext(entries))
}

// reportViolation emits one "<name> <reason>" line for a fail-closed refusal.
func reportViolation(stderr io.Writer, name string, err error) {
	var refusal *ViolationError
	if stderr == nil || !errors.As(err, &refusal) {
		return
	}
	fmt.Fprintf(stderr, "%s %s\n", name, refusal.Reason)
}

// decodePayload reads a bounded stdin payload and decodes it as JSON.
func decodePayload(stdin io.Reader) (hookPayload, error) {
	var payload hookPayload
	if stdin == nil {
		return payload, errors.New("no stdin")
	}
	data, err := io.ReadAll(io.LimitReader(stdin, maxStdinBytes))
	if err != nil {
		return payload, err
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		return payload, err
	}
	return payload, nil
}

// compiledRule is a manifest rule with its regexes compiled.
type compiledRule struct {
	rule       ManifestRule
	matcher    *regexp.Regexp
	conditions []*regexp.Regexp
}

// matchRules returns the manifest rules whose event, tool matcher, and at least
// one condition match, ordered by rule name. Every stored regex is compiled
// first, so a single uncompilable regex fails the whole run open rather than
// matching a partial rule set (REQ-CONDRULE-FIRE-03). Matching uses the Go
// regexp RE2 engine, so match time stays linear in subject length.
//
// @AX:WARN [AUTO] @AX:SPEC: SPEC-CONDRULE-001: all-or-nothing regex compilation — one bad stored regex suppresses every rule in the run rather than injecting a partial set.
// @AX:REASON: Compiling lazily inside the match loop would let an attacker-supplied manifest entry silently disable the rules sorted after it.
func matchRules(manifest Manifest, event, tool, subject string) ([]ManifestRule, error) {
	compiled, err := compileRules(manifest)
	if err != nil {
		return nil, err
	}

	var matched []ManifestRule
	for _, item := range compiled {
		if item.rule.Event != event {
			continue
		}
		if item.matcher != nil && !item.matcher.MatchString(tool) {
			continue
		}
		for _, condition := range item.conditions {
			if condition.MatchString(subject) {
				matched = append(matched, item.rule)
				break
			}
		}
	}
	sort.SliceStable(matched, func(i, j int) bool { return matched[i].Name < matched[j].Name })
	return matched, nil
}

// compileRules compiles every stored matcher and condition regex up front.
func compileRules(manifest Manifest) ([]compiledRule, error) {
	compiled := make([]compiledRule, 0, len(manifest.Rules))
	for _, rule := range manifest.Rules {
		matcher, err := compileMatcher(rule.Matcher)
		if err != nil {
			return nil, err
		}
		conditions := make([]*regexp.Regexp, 0, len(rule.Conditions))
		for _, condition := range rule.Conditions {
			expr, err := regexp.Compile(condition)
			if err != nil {
				return nil, err
			}
			conditions = append(conditions, expr)
		}
		compiled = append(compiled, compiledRule{rule: rule, matcher: matcher, conditions: conditions})
	}
	return compiled, nil
}

// compileMatcher compiles a tool matcher as a whole-name match over a literal
// alternation. An empty matcher matches every tool, which is the Claude Code
// convention.
//
// The matcher is manifest input, so it is not a regex the caller may write.
// Each `|` separated token is escaped with regexp.QuoteMeta before the
// alternation is rebuilt and anchored, which is what the compiler actually
// needs: it only ever emits literal tool names, either `Bash` or the
// `Edit|Write|MultiEdit` triple from pkg/rulecond/scope.go.
//
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-CONDRULE-001: matcher containment — anchoring alone is not sufficient because the matcher text is untrusted.
// @AX:REASON: Concatenating an unescaped matcher into `^(?:...)$` lets a manifest close the group early, as `Bash)|(.*` does, and fire a Bash-scoped rule on Write; escaping each token first is what makes the anchor hold.
func compileMatcher(matcher string) (*regexp.Regexp, error) {
	if strings.TrimSpace(matcher) == "" {
		return nil, nil
	}

	tokens := strings.Split(matcher, "|")
	quoted := make([]string, 0, len(tokens))
	for _, token := range tokens {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		quoted = append(quoted, regexp.QuoteMeta(token))
	}
	if len(quoted) == 0 {
		return nil, nil
	}
	return regexp.Compile("^(?:" + strings.Join(quoted, "|") + ")$")
}

// reportRejectedNames emits one "<sanitized-name> bad_name" line per rule the
// manifest loader refused, matching the REQ-CONDRULE-FIRE-10 stderr shape.
func reportRejectedNames(rejected []string, stderr io.Writer) {
	if stderr == nil {
		return
	}
	for _, name := range rejected {
		fmt.Fprintf(stderr, "%s %s\n", name, ReasonBadName)
	}
}

// readBodies reads each matched rule body under the conditional body root,
// suppressing a rule that fails containment and reporting only its name and
// reason code on stderr (REQ-CONDRULE-FIRE-10).
func readBodies(root string, rules []ManifestRule, stderr io.Writer) []injected {
	// A relocated body root compromises the frame every matched rule is measured
	// against, so each matched rule is suppressed and named. Reporting per rule
	// rather than once keeps the fail-closed stderr shape uniform.
	bodyRoot, err := ResolveBodyRoot(root)
	if err != nil {
		for _, rule := range rules {
			reportViolation(stderr, rule.Name, err)
		}
		return nil
	}

	entries := make([]injected, 0, len(rules))
	for _, rule := range rules {
		data, readErr := ReadBody(bodyRoot, rule.Body)
		if readErr != nil {
			reportViolation(stderr, rule.Name, readErr)
			continue
		}
		entries = append(entries, injected{name: rule.Name, body: string(data)})
	}
	return entries
}
