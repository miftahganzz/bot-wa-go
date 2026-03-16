package group

import "meow/plugins/api"

func (p *Plugin) handleLevelUp(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	if len(ctx.Args) == 0 {
		p.bot.Reply(ctx.Msg, "LevelUp grup: "+onOffText(p.bot.GetGroupConfig(ctx.Msg.Info.Chat).LevelUp))
		return true
	}
	on, ok := p.bot.ParseOnOff(ctx.Args[0])
	if !ok {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"levelup on")
		return true
	}
	if err := p.bot.SetGroupFeature(ctx.Msg.Info.Chat, func(gc *api.GroupConfig) { gc.LevelUp = on }); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan setting levelup grup")
		return true
	}
	p.bot.Reply(ctx.Msg, "LevelUp grup: "+onOffText(on))
	return true
}
