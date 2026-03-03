package group

import (
	"log"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleHideTag(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	extra := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if extra == "" {
		p.bot.Reply(ctx.Msg, "Contoh: "+ctx.Prefix+"hidetag info rapat jam 8")
		return true
	}
	if err := p.bot.SendTagAll(ctx.Msg, extra, true); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal kirim hidetag")
		log.Printf("gagal hidetag: %v", err)
	}
	return true
}
