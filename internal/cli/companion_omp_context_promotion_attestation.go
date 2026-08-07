package cli

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const maxOMPContextPromotionReportInputBytes = 512 * 1024

type companionOMPContextPromotionAttestationOptions struct {
	reportPath string
	issuedAt   string
	notBefore  string
	expiresAt  string
	validFor   time.Duration
	outputPath string
}

func newCompanionOMPContextPromotionAttestationCmd() *cobra.Command {
	var options companionOMPContextPromotionAttestationOptions
	command := &cobra.Command{
		Use:          "omp-context-promotion-attestation",
		Short:        "Sign a canonical OMP context promotion report using a stdin key",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCompanionOMPContextPromotionAttestation(command, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.reportPath, "report", "", "Canonical promotion-report-v1 JSON file")
	flags.StringVar(&options.issuedAt, "issued-at", "", "Canonical UTC issuance time")
	flags.StringVar(&options.notBefore, "not-before", "", "Canonical UTC validity start")
	flags.StringVar(&options.expiresAt, "expires-at", "", "Canonical UTC expiry time")
	flags.DurationVar(&options.validFor, "valid-for", 0, "Validity duration derived from one current UTC timestamp")
	flags.StringVar(&options.outputPath, "output", "", "New canonical promotion attestation output file")
	for _, name := range []string{"report", "output"} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func runCompanionOMPContextPromotionAttestation(
	command *cobra.Command,
	options companionOMPContextPromotionAttestationOptions,
) error {
	reportPath, outputPath, err := resolveOMPContextPromotionAttestationPaths(
		options.reportPath,
		options.outputPath,
	)
	if err != nil {
		return err
	}
	reportBytes, err := readStableOMPContextPromotionReport(reportPath)
	if err != nil {
		return err
	}
	issuedAt, notBefore, expiresAt, err := resolveOMPContextPromotionAttestationWindow(options, time.Now())
	if err != nil {
		return err
	}
	privateKey, err := readPrivateKey(command.InOrStdin())
	if err != nil {
		return err
	}
	defer clear(privateKey)
	attestationBytes, err := promptlayer.SignOMPContextPromotionAttestationV2(
		promptlayer.OMPContextPromotionAttestationSignInputV2{
			ReportBytes: reportBytes,
			IssuedAt:    issuedAt,
			NotBefore:   notBefore,
			ExpiresAt:   expiresAt,
		},
		privateKey,
	)
	if err != nil {
		return err
	}
	return writeNewPrivateOMPContextPromotionAttestation(outputPath, attestationBytes)
}
func resolveOMPContextPromotionAttestationWindow(
	options companionOMPContextPromotionAttestationOptions,
	now time.Time,
) (string, string, string, error) {
	explicit := options.issuedAt != "" || options.notBefore != "" || options.expiresAt != ""
	if options.validFor > 0 {
		if explicit || options.validFor > 24*time.Hour || now.IsZero() {
			return "", "", "", errors.New("OMP context promotion attestation validity window is invalid")
		}
		issuedAt := now.UTC()
		return issuedAt.Format(time.RFC3339Nano), issuedAt.Format(time.RFC3339Nano),
			issuedAt.Add(options.validFor).Format(time.RFC3339Nano), nil
	}
	if options.validFor < 0 || !explicit ||
		options.issuedAt == "" || options.notBefore == "" || options.expiresAt == "" {
		return "", "", "", errors.New("OMP context promotion attestation validity window is incomplete")
	}
	return options.issuedAt, options.notBefore, options.expiresAt, nil
}

func resolveOMPContextPromotionAttestationPaths(reportPath, outputPath string) (string, string, error) {
	report, err := filepath.Abs(filepath.Clean(reportPath))
	if err != nil {
		return "", "", errors.New("resolve OMP context promotion report path")
	}
	output, err := filepath.Abs(filepath.Clean(outputPath))
	if err != nil {
		return "", "", errors.New("resolve OMP context promotion attestation output path")
	}
	if report == output {
		return "", "", errors.New("OMP context promotion report and attestation output must be distinct")
	}
	if err := validateOMPContextPromotionParent(filepath.Dir(report)); err != nil {
		return "", "", errors.New("OMP context promotion report directory is unsafe")
	}
	if err := validateOMPContextPromotionParent(filepath.Dir(output)); err != nil {
		return "", "", errors.New("OMP context promotion attestation output directory is unsafe")
	}
	if info, err := os.Lstat(output); err == nil || !errors.Is(err, os.ErrNotExist) {
		if err == nil && info != nil {
			return "", "", errors.New("OMP context promotion attestation output already exists")
		}
		return "", "", errors.New("inspect OMP context promotion attestation output")
	}
	return report, output, nil
}

func validateOMPContextPromotionParent(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("unsafe parent directory")
	}
	return nil
}

func readStableOMPContextPromotionReport(path string) (body []byte, resultErr error) {
	parentPath, name := filepath.Dir(path), filepath.Base(path)
	root, err := os.OpenRoot(parentPath)
	if err != nil {
		return nil, errors.New("open OMP context promotion report directory")
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	initial, err := root.Lstat(name)
	if err != nil || !initial.Mode().IsRegular() || initial.Mode()&os.ModeSymlink != 0 ||
		initial.Size() <= 0 || initial.Size() > maxOMPContextPromotionReportInputBytes {
		return nil, errors.New("OMP context promotion report must be a bounded regular file")
	}
	file, err := root.Open(name)
	if err != nil {
		return nil, errors.New("open OMP context promotion report")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !opened.Mode().IsRegular() || !os.SameFile(initial, opened) {
		return nil, errors.New("OMP context promotion report identity changed while opening")
	}
	first, err := io.ReadAll(io.LimitReader(file, maxOMPContextPromotionReportInputBytes+1))
	if err != nil || len(first) == 0 || len(first) > maxOMPContextPromotionReportInputBytes {
		return nil, errors.New("read OMP context promotion report")
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, errors.New("rewind OMP context promotion report")
	}
	second, err := io.ReadAll(io.LimitReader(file, maxOMPContextPromotionReportInputBytes+1))
	current, statErr := root.Lstat(name)
	openedAfter, openedErr := file.Stat()
	if err != nil || !bytes.Equal(first, second) || statErr != nil || openedErr != nil ||
		!current.Mode().IsRegular() || !openedAfter.Mode().IsRegular() ||
		!os.SameFile(initial, current) || !os.SameFile(initial, openedAfter) {
		return nil, errors.New("OMP context promotion report changed while reading")
	}
	return first, nil
}

func writeNewPrivateOMPContextPromotionAttestation(path string, body []byte) (resultErr error) {
	parentPath, name := filepath.Dir(path), filepath.Base(path)
	parentBefore, err := os.Lstat(parentPath)
	if err != nil || !parentBefore.IsDir() || parentBefore.Mode()&os.ModeSymlink != 0 {
		return errors.New("OMP context promotion attestation output directory is unsafe")
	}
	root, err := os.OpenRoot(parentPath)
	if err != nil {
		return errors.New("open OMP context promotion attestation output directory")
	}
	defer func() { resultErr = errors.Join(resultErr, root.Close()) }()
	openedParent, err := root.Stat(".")
	if err != nil || !os.SameFile(parentBefore, openedParent) {
		return errors.New("OMP context promotion attestation output directory changed")
	}
	if _, err := root.Lstat(name); err == nil || !errors.Is(err, os.ErrNotExist) {
		return errors.New("OMP context promotion attestation output already exists")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return errors.New("prepare OMP context promotion attestation output")
	}
	stageName := "." + name + ".stage-" + hex.EncodeToString(nonce[:])
	stage, err := root.OpenFile(stageName, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return errors.New("create OMP context promotion attestation stage")
	}
	stageOpen := true
	defer func() {
		if stageOpen {
			_ = stage.Close()
		}
		_ = root.Remove(stageName)
	}()
	if err := stage.Chmod(0o600); err != nil {
		return errors.New("secure OMP context promotion attestation stage")
	}
	if _, err := io.Copy(stage, bytes.NewReader(body)); err != nil {
		return errors.New("write OMP context promotion attestation stage")
	}
	if err := stage.Sync(); err != nil {
		return errors.New("sync OMP context promotion attestation stage")
	}
	if err := stage.Close(); err != nil {
		return errors.New("close OMP context promotion attestation stage")
	}
	stageOpen = false
	if _, err := root.Lstat(name); err == nil || !errors.Is(err, os.ErrNotExist) {
		return errors.New("OMP context promotion attestation output became occupied")
	}
	if err := root.Link(stageName, name); err != nil {
		return errors.New("publish OMP context promotion attestation")
	}
	stageInfo, stageErr := root.Lstat(stageName)
	outputInfo, outputErr := root.Lstat(name)
	if stageErr != nil || outputErr != nil || !outputInfo.Mode().IsRegular() ||
		outputInfo.Mode().Perm() != 0o600 || !os.SameFile(stageInfo, outputInfo) {
		_ = root.Remove(name)
		return errors.New("verify OMP context promotion attestation output")
	}
	if err := root.Remove(stageName); err != nil {
		return errors.New("remove OMP context promotion attestation stage")
	}
	directory, err := root.Open(".")
	if err != nil {
		return errors.New("open OMP context promotion attestation directory for sync")
	}
	defer directory.Close()
	if err := directory.Sync(); err != nil {
		return errors.New("sync OMP context promotion attestation directory")
	}
	return nil
}
