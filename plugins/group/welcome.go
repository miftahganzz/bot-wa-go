package group

import "meow/plugins/api"

func (p *Plugin) handleWelcome(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	if len(ctx.Args) == 0 {
		p.bot.Reply(ctx.Msg, "Welcome: "+onOffText(p.bot.GetGroupConfig(ctx.Msg.Info.Chat).Welcome))
		return true
	}
	on, ok := p.bot.ParseOnOff(ctx.Args[0])
	if !ok {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"welcome on")
		return true
	}
	if err := p.bot.SetGroupFeature(ctx.Msg.Info.Chat, func(gc *api.GroupConfig) { gc.Welcome = on }); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan setting welcome")
		return true
	}
	p.bot.Reply(ctx.Msg, "Welcome: "+onOffText(on))
	return true
}
