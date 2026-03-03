package owner

import (
	"fmt"
	"log"

	"meow/plugins/api"
)

func (p *Plugin) handleAutoRead(ctx *api.Context) bool {
	if len(ctx.Args) == 0 {
		p.bot.Reply(ctx.Msg, fmt.Sprintf("AutoRead saat ini: %s", onOffText(p.bot.GetAutoRead())))
		return true
	}
	if !p.bot.IsOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner")
		return true
	}
	on, ok := p.bot.ParseOnOff(ctx.Args[0])
	if !ok {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"autoread on")
		return true
	}
	if err := p.bot.SetAutoRead(on); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan autoread")
		log.Printf("gagal simpan autoread: %v", err)
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("AutoRead: %s", onOffText(on)))
	return true
}
