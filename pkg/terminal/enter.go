package terminal

func isEnterCommand(command string) bool {
	return command == "\n" || command == "\r" || command == "\r\n"
}
