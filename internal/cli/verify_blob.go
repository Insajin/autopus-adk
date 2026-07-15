package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"path"
	"strings"
)

func collectBlobVisualEvidence(output []byte) (visualEvidence, bool) {
	if len(output) < 2 || string(output[:2]) != "PK" {
		return visualEvidence{}, false
	}
	archive, ok := readSafeBlobReport(output)
	if !ok {
		return visualEvidence{}, true
	}
	collector := newBlobCollector(archive)
	if !collector.collect(archive.Report) {
		return visualEvidence{}, true
	}
	collector.finalize()
	return collector.evidence, true
}

func (collector *blobCollector) collect(report []byte) bool {
	scanner := bufio.NewScanner(bytes.NewReader(report))
	scanner.Buffer(make([]byte, 64<<10), maxBlobLineBytes)
	events := 0
	for scanner.Scan() {
		events++
		if events > maxBlobEvents {
			return false
		}
		var event blobEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil || !collector.consume(event) {
			return false
		}
	}
	return scanner.Err() == nil
}

func (collector *blobCollector) consume(event blobEvent) bool {
	switch event.Method {
	case "onConfigure":
		var params struct {
			Config struct {
				RootDir string `json:"rootDir"`
			} `json:"config"`
		}
		if json.Unmarshal(event.Params, &params) != nil {
			return false
		}
		collector.rootDir = params.Config.RootDir
	case "onProject":
		return collector.consumeProject(event.Params)
	case "onTestBegin":
		return collector.consumeTestBegin(event.Params)
	case "onStepBegin":
		return collector.consumeStepBegin(event.Params)
	case "onStepEnd":
		return collector.consumeStepEnd(event.Params)
	case "onTestEnd":
		return collector.consumeTestEnd(event.Params)
	case "onAttach":
		return collector.consumeAttachments(event.Params)
	}
	return true
}

func (collector *blobCollector) consumeProject(raw json.RawMessage) bool {
	var params struct {
		Project struct {
			Name        string      `json:"name"`
			SnapshotDir string      `json:"snapshotDir"`
			Suites      []blobSuite `json:"suites"`
		} `json:"project"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return false
	}
	info := blobTestInfo{Project: params.Project.Name, SnapshotDir: params.Project.SnapshotDir}
	for _, suite := range params.Project.Suites {
		if !collector.indexSuite(suite, info, 0) {
			return false
		}
	}
	return true
}

func (collector *blobCollector) consumeTestBegin(raw json.RawMessage) bool {
	var params struct {
		TestID string `json:"testId"`
		Result struct {
			ID    string `json:"id"`
			Retry int    `json:"retry"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return false
	}
	if params.TestID == "" || params.Result.ID == "" {
		return true
	}
	if _, duplicate := collector.results[params.Result.ID]; duplicate {
		return false
	}
	collector.resultOrder++
	collector.results[params.Result.ID] = &blobResult{
		TestID: params.TestID, ResultID: params.Result.ID, Retry: params.Result.Retry, Order: collector.resultOrder,
	}
	collector.resultSequence = append(collector.resultSequence, collector.results[params.Result.ID])
	if project := collector.tests[params.TestID].Project; project != "" {
		collector.projectSet[project] = struct{}{}
	}
	return true
}

func (collector *blobCollector) indexSuite(suite blobSuite, info blobTestInfo, depth int) bool {
	if depth > 128 {
		return false
	}
	for _, entry := range suite.Entries {
		if !collector.indexSuiteEntry(entry, info, depth+1) {
			return false
		}
	}
	return true
}

func (collector *blobCollector) indexSuiteEntry(entry blobSuiteEntry, info blobTestInfo, depth int) bool {
	if depth > 128 {
		return false
	}
	if entry.TestID != "" {
		collector.tests[entry.TestID] = info
	}
	for _, nested := range entry.Entries {
		if !collector.indexSuiteEntry(nested, info, depth+1) {
			return false
		}
	}
	return true
}

func (collector *blobCollector) consumeStepBegin(raw json.RawMessage) bool {
	var params struct {
		TestID   string `json:"testId"`
		ResultID string `json:"resultId"`
		Step     struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Category string `json:"category"`
		} `json:"step"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return false
	}
	result := collector.results[params.ResultID]
	if result == nil || params.Step.ID == "" {
		return true
	}
	name, anonymous, ok := screenshotNameFromStep(params.Step.Category, params.Step.Title)
	if !ok {
		return true
	}
	if anonymous {
		result.AnonymousSnapshots++
		name = fmt.Sprintf("anonymous-screenshot-%d.png", result.AnonymousSnapshots)
	}
	if collector.stepsByID[params.ResultID] == nil {
		collector.stepsByID[params.ResultID] = map[string]*blobStep{}
	}
	if _, duplicate := collector.stepsByID[params.ResultID][params.Step.ID]; duplicate {
		return false
	}
	step := &blobStep{
		TestID: params.TestID, ResultID: params.ResultID, Name: name, Anonymous: anonymous,
	}
	collector.stepsByID[params.ResultID][params.Step.ID] = step
	collector.stepsByResult[params.ResultID] = append(collector.stepsByResult[params.ResultID], step)
	return true
}

func (collector *blobCollector) consumeStepEnd(raw json.RawMessage) bool {
	var params struct {
		ResultID string `json:"resultId"`
		Step     struct {
			ID    string          `json:"id"`
			Error json.RawMessage `json:"error"`
		} `json:"step"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return false
	}
	if step := collector.stepsByID[params.ResultID][params.Step.ID]; step != nil {
		step.Ended = true
		step.Failed = len(params.Step.Error) > 0 && string(params.Step.Error) != "null"
	}
	return true
}

func (collector *blobCollector) consumeTestEnd(raw json.RawMessage) bool {
	var params struct {
		Test struct {
			TestID string `json:"testId"`
		} `json:"test"`
		Result struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"result"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return false
	}
	result := collector.results[params.Result.ID]
	if result == nil {
		return true
	}
	result.TestID = firstNonEmpty(result.TestID, params.Test.TestID)
	result.Status = params.Result.Status
	result.Ended = true
	return true
}

func (collector *blobCollector) consumeAttachments(raw json.RawMessage) bool {
	var params struct {
		TestID      string                 `json:"testId"`
		ResultID    string                 `json:"resultId"`
		Attachments []playwrightAttachment `json:"attachments"`
		Attachment  *playwrightAttachment  `json:"attachment"`
	}
	if json.Unmarshal(raw, &params) != nil {
		return false
	}
	if params.Attachment != nil {
		params.Attachments = append(params.Attachments, *params.Attachment)
	}
	result := collector.results[params.ResultID]
	if result == nil {
		return true
	}
	for _, attachment := range params.Attachments {
		if !safeBlobEntryName(attachment.Path) {
			continue
		}
		if _, exists := collector.entries[attachment.Path]; !exists {
			continue
		}
		result.Attachments = append(result.Attachments, attachment)
	}
	return true
}

func portableBase(value string) string {
	return path.Base(strings.ReplaceAll(value, "\\", "/"))
}
