package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type companionOMPContextStaticPolicyOptions struct {
	reportPath            string
	target                string
	promotionSigningKeyID string
	releaseLineageKeyID   string
	releaseLineageHandoff string
	minimumRollbackFloor  uint64
}

func newCompanionOMPContextStaticPolicyCmd() *cobra.Command {
	var options companionOMPContextStaticPolicyOptions
	command := &cobra.Command{
		Use:          "omp-context-static-policy",
		Short:        "Derive canonical release policy from production OMP evidence",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCompanionOMPContextStaticPolicy(command, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.reportPath, "report", "", "Canonical OMP context promotion report")
	flags.StringVar(&options.target, "target", "", "Release target bound into active authority")
	flags.StringVar(&options.promotionSigningKeyID, "promotion-signing-key-id", "", "Promotion attestation key identity")
	flags.StringVar(&options.releaseLineageKeyID, "release-lineage-key-id", "", "Release lineage key identity")
	flags.StringVar(&options.releaseLineageHandoff, "release-lineage-handoff", "", "Release lineage handoff contract")
	flags.Uint64Var(&options.minimumRollbackFloor, "minimum-rollback-floor", 0, "Minimum accepted release rollback floor")
	for _, name := range []string{
		"report", "target", "promotion-signing-key-id", "release-lineage-key-id",
		"release-lineage-handoff", "minimum-rollback-floor",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func runCompanionOMPContextStaticPolicy(
	command *cobra.Command,
	options companionOMPContextStaticPolicyOptions,
) error {
	body, err := readStableOMPContextPromotionReport(options.reportPath)
	if err != nil {
		return err
	}
	if rejectDuplicatePipelineOMPJSON(body) != nil {
		return errors.New("OMP context promotion report JSON is not unique")
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	var report promptlayer.OMPContextPromotionReportV1
	if decoder.Decode(&report) != nil || requireOMPContextStaticPolicyEOF(decoder) != nil {
		return errors.New("OMP context promotion report JSON is invalid")
	}
	_, canonical, err := promptlayer.BuildOMPContextPromotionReportV1(report)
	if err != nil || !bytes.Equal(canonical, body) {
		return errors.New("OMP context promotion report is not canonical")
	}
	staticPolicy, policy, err := promptlayer.BuildOMPContextPromotionStaticPolicyV3(
		report,
		options.target,
		options.promotionSigningKeyID,
		options.releaseLineageKeyID,
		options.releaseLineageHandoff,
		options.minimumRollbackFloor,
	)
	if err != nil {
		return err
	}
	if err := promptlayer.ValidateOMPContextPromotionActiveStaticPolicyV3(staticPolicy); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(command.OutOrStdout(), base64.RawURLEncoding.EncodeToString(policy)); err != nil {
		return errors.New("write OMP context promotion static policy")
	}
	return nil
}

func requireOMPContextStaticPolicyEOF(decoder *json.Decoder) error {
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return errors.New("trailing OMP context promotion report JSON")
	}
	return nil
}
