package omp

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

func decodeOMPExactJSON(data []byte, target any) bool {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if decoder.Decode(target) != nil {
		return false
	}
	var extra any
	return errors.Is(decoder.Decode(&extra), io.EOF)
}

func isOMPOutputLimitError(err error) bool {
	return errors.Is(err, processprobe.ErrOutputLimit)
}
