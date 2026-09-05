package cli

import "strings"

// upsertArgValue sets flag to value exactly once, replacing any existing
// occurrence and stopping at a "--" separator.
func upsertArgValue(args []string, flag, value string) []string {
	result := make([]string, 0, len(args)+2)
	found := false
	for index := 0; index < len(args); index++ {
		if args[index] == "--" {
			if !found {
				result = append(result, flag, value)
			}
			return append(result, args[index:]...)
		}
		switch {
		case args[index] == flag:
			if !found {
				result = append(result, flag, value)
				found = true
			}
			if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
				index++
			}
		case strings.HasPrefix(args[index], flag+"="):
			if !found {
				result = append(result, flag+"="+value)
				found = true
			}
		default:
			result = append(result, args[index])
		}
	}
	if !found {
		result = append(result, flag, value)
	}
	return result
}

// ensureBoolArg adds a boolean flag exactly once, deduplicating existing
// occurrences and stopping at a "--" separator.
func ensureBoolArg(args []string, flag string) []string {
	result := make([]string, 0, len(args)+1)
	found := false
	for index, arg := range args {
		if arg == "--" {
			if !found {
				result = append(result, flag)
			}
			return append(result, args[index:]...)
		}
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			if !found {
				result = append(result, flag)
				found = true
			}
			continue
		}
		result = append(result, arg)
	}
	if !found {
		result = append(result, flag)
	}
	return result
}
