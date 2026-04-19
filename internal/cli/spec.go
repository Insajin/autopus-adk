package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/spec"
)

// newSpecCmd는 spec 서브커맨드를 생성한다.
func newSpecCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "spec",
		Short: "SPEC document management",
	}

	cmd.AddCommand(newSpecNewCmd())
	cmd.AddCommand(newSpecSelfVerifyCmd())
	cmd.AddCommand(newSpecValidateCmd())
	cmd.AddCommand(newSpecReviewCmd())
	return cmd
}

func newSpecNewCmd() *cobra.Command {
	var title string

	cmd := &cobra.Command{
		Use:   "new <id>",
		Short: "Scaffold a new SPEC document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			if title == "" {
				title = id
			}

			if err := spec.Scaffold(".", id, title); err != nil {
				return fmt.Errorf("SPEC 생성 실패: %w", err)
			}

			fmt.Printf("SPEC-%s 생성 완료: .autopus/specs/SPEC-%s/\n", id, id)
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "SPEC title")
	return cmd
}

func newSpecValidateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate <spec-dir>",
		Short: "Validate a SPEC document",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			doc, err := spec.Load(args[0])
			if err != nil {
				return fmt.Errorf("SPEC 로드 실패: %w", err)
			}

			errs := spec.ValidateSpec(doc)
			if len(errs) == 0 {
				fmt.Println("SPEC 검증 통과")
				return nil
			}

			hasError := false
			for _, e := range errs {
				level := e.Level
				fmt.Fprintf(os.Stderr, "[%s] %s: %s\n", level, e.Field, e.Message)
				if level == "error" {
					hasError = true
				}
			}

			if hasError {
				return fmt.Errorf("SPEC 검증 실패")
			}
			fmt.Println("SPEC 검증 완료 (경고 있음)")
			return nil
		},
	}
}
