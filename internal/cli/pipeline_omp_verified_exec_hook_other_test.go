//go:build !darwin && !linux

package cli

func configurePipelineOMPVerifiedExecDarwinStopForTest(
	*pipelineOMPVerifiedExecCommand,
	func(),
) bool {
	return false
}
