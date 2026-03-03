package group

import "meow/plugins/api"

func (p *Plugin) handleGoodbye(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	if len(ctx.Args) == 0 {
		p.bot.Reply(ctx.Msg, "Goodbye: "+onOffText(p.bot.GetGroupConfig(ctx.Msg.Info.Chat).Goodbye))
		return true
	}
	on, ok := p.bot.ParseOnOff(ctx.Args[0])
	if !ok {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"goodbye on")
		return true
	}
	if err := p.bot.SetGroupFeature(ctx.Msg.Info.Chat, func(gc *api.GroupConfig) { gc.Goodbye = on }); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan setting goodbye")
		return true
	}
	p.bot.Reply(ctx.Msg, "Goodbye: "+onOffText(on))
	return true
}
