//go:build darwin || linux

package cli

func configurePipelineOMPVerifiedExecDarwinStopForTest(
	command *pipelineOMPVerifiedExecCommand,
	afterStop func(),
) bool {
	command.afterFirstDarwinStop = afterStop
	return true
}
