package owner

import (
	"log"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleSetPrefix(ctx *api.Context) bool {
	if !p.bot.IsOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner")
		return true
	}
	newPrefixes, err := p.bot.ParsePrefixArgs(ctx.Args)
	if err != nil {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"setprefix .,!,#")
		return true
	}
	if err := p.bot.SetPrefixes(newPrefixes); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan prefix")
		log.Printf("gagal simpan prefix: %v", err)
		return true
	}
	p.bot.Reply(ctx.Msg, "Prefix baru: "+strings.Join(newPrefixes, " "))
	return true
}
