// Package hooks_test contains regression tests for .claude/hooks/task-created-validate.sh.
// This file defines the 50-case synthetic payload table (QG2).
package hooks_test

// tcase describes a single test vector for the TaskCreated validation hook.
type tcase struct {
	name        string // unique test name
	input       string // raw stdin JSON payload
	mode        string // "warn" | "enforce"
	wantExit    int    // expected process exit code
	wantOutcome string // expected audit log outcome: "pass" | "warn" | "fail"
}

// validCases holds 20 payloads that must always exit 0 with outcome=pass.
// Each uses a distinct allow-list verb to exercise full coverage of the verb table.
var validCases = []tcase{
	// verb: add
	{name: "valid_add_new_feature", input: `{"task_subject":"add new feature"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	{name: "valid_add_enforce", input: `{"task_subject":"add new feature"}`, mode: "enforce", wantExit: 0, wantOutcome: "pass"},
	// verb: update
	{name: "valid_update_user_config", input: `{"task_subject":"update user config"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	{name: "valid_update_enforce", input: `{"task_subject":"update auth flow"}`, mode: "enforce", wantExit: 0, wantOutcome: "pass"},
	// verb: remove
	{name: "valid_remove_legacy_module", input: `{"task_subject":"remove legacy module"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: fix
	{name: "valid_fix_auth_race", input: `{"task_subject":"fix auth race condition"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: refactor
	{name: "valid_refactor_db_pool", input: `{"task_subject":"refactor db connection pool"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: rename
	{name: "valid_rename_variable", input: `{"task_subject":"rename variable X to Y"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: rewrite
	{name: "valid_rewrite_parser", input: `{"task_subject":"rewrite parser stage 2"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: implement
	{name: "valid_implement_cache", input: `{"task_subject":"implement cache layer"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: create
	{name: "valid_create_dashboard", input: `{"task_subject":"create dashboard panel"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: delete
	{name: "valid_delete_obsolete", input: `{"task_subject":"delete obsolete file"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: clean
	{name: "valid_clean_build", input: `{"task_subject":"clean build artifacts"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: check
	{name: "valid_check_schema", input: `{"task_subject":"check schema validity"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: verify
	{name: "valid_verify_migration", input: `{"task_subject":"verify migration script"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: document
	{name: "valid_document_api", input: `{"task_subject":"document api endpoints"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: migrate
	{name: "valid_migrate_v1_to_v2", input: `{"task_subject":"migrate v1 to v2"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: split
	{name: "valid_split_service", input: `{"task_subject":"split service into modules"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: merge
	{name: "valid_merge_helpers", input: `{"task_subject":"merge duplicate helpers"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
	// verb: optimize (title >= 5 chars, valid verb)
	{name: "valid_optimize_queue", input: `{"task_subject":"optimize queue throughput"}`, mode: "warn", wantExit: 0, wantOutcome: "pass"},
}

// titleTooShortViolations holds 10 payloads with title length < 5.
// warn  → exit 0, outcome=warn
// enforce → exit 2, outcome=fail
var titleTooShortViolations = []tcase{
	{name: "short_empty", input: `{"task_subject":""}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "short_a", input: `{"task_subject":"a"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "short_hi", input: `{"task_subject":"hi"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "short_fix", input: `{"task_subject":"fix"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "short_add", input: `{"task_subject":"add"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	// enforce variants — same titles, now exit 2
	{name: "short_empty_enforce", input: `{"task_subject":""}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "short_a_enforce", input: `{"task_subject":"a"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "short_hi_enforce", input: `{"task_subject":"hi"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "short_fix_enforce", input: `{"task_subject":"fix"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "short_add_enforce", input: `{"task_subject":"add"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
}

// badVerbViolations holds 10 payloads whose first word is not in the allow-list.
// warn  → exit 0, outcome=warn
// enforce → exit 2, outcome=fail
var badVerbViolations = []tcase{
	{name: "badverb_quickfix_warn", input: `{"task_subject":"quickfix thing here"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "badverb_hotfix_warn", input: `{"task_subject":"hotfix bug now"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "badverb_patch_warn", input: `{"task_subject":"patch security issue"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "badverb_hack_warn", input: `{"task_subject":"hackish workaround here"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "badverb_misc_warn", input: `{"task_subject":"misc update flow thing"}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	// enforce variants
	{name: "badverb_quickfix_enforce", input: `{"task_subject":"quickfix thing here"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "badverb_hotfix_enforce", input: `{"task_subject":"hotfix bug now"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "badverb_patch_enforce", input: `{"task_subject":"patch security issue"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "badverb_wtf_enforce", input: `{"task_subject":"wtfness is here"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "badverb_stuff_enforce", input: `{"task_subject":"stuff and things"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
}

// invalidJSONViolations holds 10 payloads that fail JSON parsing or missing required fields.
// Both modes produce outcome=warn (warn) or outcome=fail (enforce) with invalid_payload reason.
var invalidJSONViolations = []tcase{
	{name: "json_not_json_warn", input: `not-json`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "json_incomplete_warn", input: `{incomplete`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "json_empty_object_warn", input: `{}`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "json_array_warn", input: `[]`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	{name: "json_null_warn", input: `null`, mode: "warn", wantExit: 0, wantOutcome: "warn"},
	// enforce variants
	{name: "json_not_json_enforce", input: `not-json`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "json_incomplete_enforce", input: `{incomplete`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "json_empty_object_enforce", input: `{}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "json_no_title_enforce", input: `{"task_description":"no title field"}`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
	{name: "json_array_enforce", input: `[]`, mode: "enforce", wantExit: 2, wantOutcome: "fail"},
}
