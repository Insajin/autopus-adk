package cli

// playwrightResult is a minimal subset of Playwright JSON reporter output.
type playwrightResult struct {
	Suites []playwrightSuite `json:"suites"`
}

// playwrightSuite holds test suite data from Playwright JSON output.
type playwrightSuite struct {
	Specs []playwrightSpec `json:"specs"`
}

// playwrightSpec holds individual test spec data including attachments.
type playwrightSpec struct {
	Tests []playwrightTest `json:"tests"`
}

// playwrightTest holds test result data from a single test run.
type playwrightTest struct {
	Results []playwrightTestResult `json:"results"`
}

// playwrightTestResult holds attachments such as screenshot paths.
type playwrightTestResult struct {
	Attachments []playwrightAttachment `json:"attachments"`
}

// playwrightAttachment represents a file attachment (e.g., screenshot) in a test result.
type playwrightAttachment struct {
	Name        string `json:"name"`
	ContentType string `json:"contentType"`
	Path        string `json:"path"`
}
