package owner

import "strings"

func normalizeOwnerCommand(cmd string) string {
	if strings.HasPrefix(cmd, "owner/") {
		return strings.TrimPrefix(cmd, "owner/")
	}
	return cmd
}

func onOffText(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}
