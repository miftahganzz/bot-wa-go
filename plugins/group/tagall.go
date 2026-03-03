package group

import (
	"log"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleTagAll(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	extra := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if err := p.bot.SendTagAll(ctx.Msg, extra, false); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal kirim tagall")
		log.Printf("gagal tagall: %v", err)
	}
	return true
}
