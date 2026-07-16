package companionmanifest

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestDecodeReleaseArchive_ConsumesGzipTrailer(t *testing.T) {
	valid := makeReleaseArchiveFixture(t)
	cases := []struct {
		name   string
		mutate func([]byte) []byte
		want   string
	}{
		{name: "corrupt_crc", want: "gzip: invalid checksum", mutate: func(data []byte) []byte {
			data[len(data)-8] ^= 0xff
			return data
		}},
		{name: "truncated_trailer", want: "unexpected EOF", mutate: func(data []byte) []byte {
			return data[:len(data)-4]
		}},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			archive := test.mutate(bytes.Clone(valid))
			if _, err := decodeReleaseArchive(bytes.NewReader(archive)); err == nil ||
				!strings.Contains(err.Error(), test.want) {
				t.Fatalf("decode error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestWriteReleaseArchiveFixture_ReportsFinalizationFailures(t *testing.T) {
	t.Run("gzip_finalize", func(t *testing.T) {
		output := &failingArchiveWriter{writeErr: errors.New("fixture write failure")}
		if err := writeReleaseArchiveFixture(output); err == nil ||
			!strings.Contains(err.Error(), "fixture write failure") {
			t.Fatalf("finalization error = %v", err)
		}
	})
	t.Run("output_close", func(t *testing.T) {
		output := &failingArchiveWriter{closeErr: errors.New("fixture close failure")}
		if err := writeReleaseArchiveFixture(output); err == nil ||
			!strings.Contains(err.Error(), "fixture close failure") {
			t.Fatalf("close error = %v", err)
		}
	})
}

func makeReleaseArchiveFixture(t *testing.T) []byte {
	t.Helper()
	output := &bufferArchiveWriter{}
	if err := writeReleaseArchiveFixture(output); err != nil {
		t.Fatal(err)
	}
	return bytes.Clone(output.Bytes())
}

func writeReleaseArchiveFixture(output io.WriteCloser) error {
	gzipWriter := gzip.NewWriter(output)
	tarWriter := tar.NewWriter(gzipWriter)
	data := []byte("fixture executable")
	writeErr := tarWriter.WriteHeader(&tar.Header{Name: "auto", Mode: 0o755, Size: int64(len(data))})
	if writeErr == nil {
		_, writeErr = tarWriter.Write(data)
	}
	return errors.Join(writeErr, tarWriter.Close(), gzipWriter.Close(), output.Close())
}

type bufferArchiveWriter struct {
	bytes.Buffer
}

func (writer *bufferArchiveWriter) Close() error { return nil }

type failingArchiveWriter struct {
	closeErr error
	writeErr error
}

func (writer *failingArchiveWriter) Write(data []byte) (int, error) {
	if writer.writeErr != nil {
		return 0, writer.writeErr
	}
	return len(data), nil
}

func (writer *failingArchiveWriter) Close() error { return writer.closeErr }
