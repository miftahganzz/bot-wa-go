package owner

import (
	"fmt"
	"strconv"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleAddLimit(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Hanya owner utama yang bisa addlimit")
		return true
	}
	if len(ctx.Args) < 1 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"addlimit 62812xxxxxx 5 / reply user: "+ctx.Prefix+"addlimit 5")
		return true
	}
	var (
		targetRaw string
		amountRaw string
	)
	if len(ctx.Args) == 1 {
		amountRaw = ctx.Args[0]
	} else {
		targetRaw = ctx.Args[0]
		amountRaw = ctx.Args[1]
	}
	phone, err := p.bot.ResolveTargetPhone(ctx.Msg, targetRaw)
	if err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	amount, err := strconv.Atoi(strings.TrimSpace(amountRaw))
	if err != nil || amount <= 0 {
		p.bot.Reply(ctx.Msg, "Jumlah limit harus angka > 0")
		return true
	}
	limit, err := p.bot.OwnerAddLimit(ctx.Msg, phone, amount)
	if err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("Limit %s ditambah %d. Total limit: %d", phone, amount, limit))
	return true
}

func (p *Plugin) handleResetLimit(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Hanya owner utama yang bisa resetlimit")
		return true
	}
	if len(ctx.Args) > 1 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"resetlimit 62812xxxxxx / reply user: "+ctx.Prefix+"resetlimit")
		return true
	}
	targetRaw := ""
	if len(ctx.Args) == 1 {
		targetRaw = ctx.Args[0]
	}
	phone, err := p.bot.ResolveTargetPhone(ctx.Msg, targetRaw)
	if err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	limit, err := p.bot.OwnerResetLimit(ctx.Msg, phone)
	if err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("Limit %s direset. Total limit: %d", phone, limit))
	return true
}

func (p *Plugin) handleDelLimit(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Hanya owner utama yang bisa dellimit")
		return true
	}
	if len(ctx.Args) > 1 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"dellimit 62812xxxxxx / reply user: "+ctx.Prefix+"dellimit")
		return true
	}
	targetRaw := ""
	if len(ctx.Args) == 1 {
		targetRaw = ctx.Args[0]
	}
	phone, err := p.bot.ResolveTargetPhone(ctx.Msg, targetRaw)
	if err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	limit, err := p.bot.OwnerDelLimit(ctx.Msg, phone)
	if err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("Limit %s dihapus. Total limit: %d", phone, limit))
	return true
}
