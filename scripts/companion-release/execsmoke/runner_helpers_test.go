//go:build darwin || linux

package main

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func replaceEnvironment(environment []string, name, value string) []string {
	prefix := name + "="
	result := make([]string, 0, len(environment)+1)
	for _, entry := range environment {
		if !strings.HasPrefix(entry, prefix) {
			result = append(result, entry)
		}
	}
	return append(result, prefix+value)
}

func assertRecordedProcessGone(t *testing.T, pidPath string) {
	t.Helper()
	contents, err := os.ReadFile(pidPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", pidPath, err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(contents)))
	if err != nil {
		t.Fatalf("recorded PID = %q: %v", contents, err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for {
		err = syscall.Kill(pid, 0)
		if errors.Is(err, syscall.ESRCH) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant PID %d still exists after process-group cleanup", pid)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
