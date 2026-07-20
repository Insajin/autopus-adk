package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

const orchestraJobWaitOutputSchema = "orchestra_job_wait.v1"

type orchestraJobWaitOutput struct {
	Schema           string              `json:"schema"`
	JobID            string              `json:"job_id"`
	Status           orchestra.JobStatus `json:"status"`
	NextRequiredStep string              `json:"next_required_step"`
}

func writeOrchestraJobWaitOutput(w io.Writer, jobID string, status orchestra.JobStatus, format string) error {
	if err := validateOrchestraOutputFormat(format); err != nil {
		return err
	}
	if format == "" || format == orchestraOutputText {
		_, err := fmt.Fprintf(w, "Job %s: %s\n", jobID, status)
		return err
	}
	return json.NewEncoder(w).Encode(orchestraJobWaitOutput{
		Schema:           orchestraJobWaitOutputSchema,
		JobID:            jobID,
		Status:           status,
		NextRequiredStep: fmt.Sprintf("auto orchestra result %s --format json", jobID),
	})
}
