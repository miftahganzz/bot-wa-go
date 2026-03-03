package owner

import "meow/plugins/api"

func (p *Plugin) handleSetOwner(ctx *api.Context) bool {
	return p.handleAddOwner(ctx)
}

func (p *Plugin) handleAddOwner(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Hanya owner utama (config.json) yang bisa addowner")
		return true
	}
	if len(ctx.Args) != 1 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"addowner 62812xxxxxx")
		return true
	}
	newOwner := p.bot.NormalizePhone(ctx.Args[0])
	if newOwner == "" {
		p.bot.Reply(ctx.Msg, "Nomor owner tidak valid")
		return true
	}
	if err := p.bot.AddOwner(ctx.Msg, newOwner); err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, "Owner tambahan ditambahkan: "+newOwner)
	return true
}
