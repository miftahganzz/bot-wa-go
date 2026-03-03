package owner

import (
	"fmt"
	"strconv"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleSetDaily(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Hanya owner utama yang bisa setdaily")
		return true
	}
	if len(ctx.Args) == 0 {
		xp, limit, cooldown := p.bot.GetDailyConfig()
		p.bot.Reply(ctx.Msg, fmt.Sprintf("Daily config saat ini\nXP: %d\nLimit: %d\nCooldown: %d jam", xp, limit, cooldown))
		return true
	}
	if len(ctx.Args) != 3 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"setdaily 120 3 24")
		return true
	}
	xp, err := strconv.ParseInt(strings.TrimSpace(ctx.Args[0]), 10, 64)
	if err != nil {
		p.bot.Reply(ctx.Msg, "XP harus angka")
		return true
	}
	limit, err := strconv.Atoi(strings.TrimSpace(ctx.Args[1]))
	if err != nil {
		p.bot.Reply(ctx.Msg, "Limit harus angka")
		return true
	}
	cooldown, err := strconv.Atoi(strings.TrimSpace(ctx.Args[2]))
	if err != nil {
		p.bot.Reply(ctx.Msg, "Cooldown jam harus angka")
		return true
	}
	if err := p.bot.OwnerSetDaily(ctx.Msg, xp, limit, cooldown); err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("Daily config diupdate\nXP: %d\nLimit: %d\nCooldown: %d jam", xp, limit, cooldown))
	return true
}
