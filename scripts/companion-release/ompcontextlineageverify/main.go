package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/insajin/autopus-adk/pkg/companionmanifest"
)

type options struct {
	lineage, signature, receiptBundle          string
	keyID, handoff, upstream, executable       string
	sourceRepository, sourceCommit, sourceTree string
	target, version                            string
	minimumRollbackFloor                       uint64
}

func main() {
	if err := run(os.Args[1:], time.Now().UTC()); err != nil {
		fmt.Fprintln(os.Stderr, "OMP context release lineage verifier: verification failed")
		os.Exit(1)
	}
}

func run(arguments []string, now time.Time) error {
	opts, err := parseOptions(arguments)
	if err != nil {
		return err
	}
	lineage, err := readRegular(opts.lineage, 16*1024)
	if err != nil {
		return err
	}
	signature, err := readRegular(opts.signature, 64)
	if err != nil || len(signature) != 64 {
		return errors.New("invalid lineage signature")
	}
	trusted, err := companionmanifest.VerifyConfiguredPublicKeyReceiptBundle(
		opts.receiptBundle,
		companionmanifest.PublicKeyReceiptPolicy{
			Now: now, ExpectedKeyID: opts.keyID, ExpectedHandoff: opts.handoff,
			MinimumRollbackFloor: opts.minimumRollbackFloor,
		},
	)
	if err != nil {
		return errors.New("release key receipt verification failed")
	}
	_, err = companionmanifest.VerifyOMPContextReleaseLineage(
		lineage,
		signature,
		companionmanifest.OMPContextReleaseLineagePolicy{
			Now: now, ExpectedKeyID: opts.keyID, ExpectedHandoff: opts.handoff,
			MinimumRollbackFloor:   opts.minimumRollbackFloor,
			ExpectedUpstreamSHA256: opts.upstream, ExpectedExecutableSHA256: opts.executable,
			ExpectedSourceRepository: opts.sourceRepository,
			ExpectedSourceCommit:     opts.sourceCommit, ExpectedSourceTree: opts.sourceTree,
			ExpectedTarget: opts.target, ExpectedVersion: opts.version,
		},
		trusted,
	)
	return err
}

func parseOptions(arguments []string) (options, error) {
	var result options
	set := flag.NewFlagSet("ompcontextlineageverify", flag.ContinueOnError)
	set.SetOutput(io.Discard)
	set.StringVar(&result.lineage, "lineage", "", "canonical lineage path")
	set.StringVar(&result.signature, "signature", "", "raw signature path")
	set.StringVar(&result.receiptBundle, "receipt-bundle", "", "release key receipt bundle")
	set.StringVar(&result.keyID, "key-id", "", "expected release key")
	set.StringVar(&result.handoff, "handoff", "", "expected handoff")
	set.Uint64Var(&result.minimumRollbackFloor, "minimum-rollback-floor", 0, "minimum rollback floor")
	set.StringVar(&result.upstream, "upstream-sha256", "", "expected U")
	set.StringVar(&result.executable, "executable-sha256", "", "expected D")
	set.StringVar(&result.sourceRepository, "source-repository", "", "source repository")
	set.StringVar(&result.sourceCommit, "source-commit", "", "source commit")
	set.StringVar(&result.sourceTree, "source-tree", "", "source tree")
	set.StringVar(&result.target, "target", "", "release target")
	set.StringVar(&result.version, "version", "", "release version")
	if err := set.Parse(arguments); err != nil || set.NArg() != 0 {
		return result, errors.New("invalid arguments")
	}
	if result.lineage == "" || result.signature == "" || result.receiptBundle == "" ||
		result.keyID == "" || result.handoff == "" || result.minimumRollbackFloor == 0 ||
		result.upstream == "" || result.executable == "" || result.sourceRepository == "" ||
		result.sourceCommit == "" || result.sourceTree == "" || result.target == "" || result.version == "" {
		return result, errors.New("required argument is missing")
	}
	return result, nil
}

func readRegular(path string, maximum int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&0o022 != 0 || info.Size() <= 0 || info.Size() > maximum {
		return nil, errors.New("lineage input is unsafe")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("open lineage input")
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, maximum+1))
	if err != nil || int64(len(body)) > maximum {
		return nil, errors.New("read lineage input")
	}
	return body, nil
}
