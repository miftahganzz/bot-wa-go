package group

import (
	"strings"

	"meow/plugins/api"
)

func normalizeGroupCommand(cmd string) string {
	if strings.HasPrefix(cmd, "group/") {
		return strings.TrimPrefix(cmd, "group/")
	}
	return cmd
}

func (p *Plugin) guardGroupAdmin(ctx *api.Context) bool {
	if !ctx.Msg.Info.IsGroup {
		p.bot.Reply(ctx.Msg, "Command ini hanya untuk grup")
		return false
	}
	if !p.bot.IsOwner(ctx.Msg) && !p.bot.IsGroupAdmin(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Khusus admin grup / owner")
		return false
	}
	return true
}

func onOffText(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}
