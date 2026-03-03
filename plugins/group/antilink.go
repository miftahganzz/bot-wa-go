package group

import "meow/plugins/api"

func (p *Plugin) handleAntiLink(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	if len(ctx.Args) == 0 {
		p.bot.Reply(ctx.Msg, "AntiLink: "+onOffText(p.bot.GetGroupConfig(ctx.Msg.Info.Chat).AntiLink))
		return true
	}
	on, ok := p.bot.ParseOnOff(ctx.Args[0])
	if !ok {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"antilink on")
		return true
	}
	if err := p.bot.SetGroupFeature(ctx.Msg.Info.Chat, func(gc *api.GroupConfig) { gc.AntiLink = on }); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan setting antilink")
		return true
	}
	p.bot.Reply(ctx.Msg, "AntiLink: "+onOffText(on))
	return true
}
