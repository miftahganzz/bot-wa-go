package owner

import (
	"fmt"
	"log"

	"meow/plugins/api"
)

func (p *Plugin) handleLevelUp(ctx *api.Context) bool {
	if len(ctx.Args) == 0 {
		p.bot.Reply(ctx.Msg, fmt.Sprintf("LevelUp global saat ini: %s", onOffText(p.bot.GetLevelUpEnabled())))
		return true
	}
	if !p.bot.IsOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner")
		return true
	}
	on, ok := p.bot.ParseOnOff(ctx.Args[0])
	if !ok {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"levelup on")
		return true
	}
	if err := p.bot.SetLevelUpEnabled(on); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan setting levelup global")
		log.Printf("gagal simpan levelup global: %v", err)
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("LevelUp global: %s", onOffText(on)))
	return true
}
