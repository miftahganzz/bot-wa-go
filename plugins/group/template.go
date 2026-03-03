package group

import (
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleSetWelcomeTemplate(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	if len(ctx.Args) == 0 {
		current := strings.TrimSpace(p.bot.GetGroupConfig(ctx.Msg.Info.Chat).WelcomeTemplate)
		if current == "" {
			p.bot.Reply(ctx.Msg, "Welcome template kosong. Contoh: "+ctx.Prefix+"setwelcome 👋 {user} welcome to {group} ({count})")
			return true
		}
		p.bot.Reply(ctx.Msg, "Welcome template:\n"+current)
		return true
	}
	tpl := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if strings.EqualFold(tpl, "reset") {
		tpl = ""
	}
	if err := p.bot.SetGroupFeature(ctx.Msg.Info.Chat, func(gc *api.GroupConfig) { gc.WelcomeTemplate = tpl }); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan welcome template")
		return true
	}
	if tpl == "" {
		p.bot.Reply(ctx.Msg, "Welcome template direset ke default.")
		return true
	}
	p.bot.Reply(ctx.Msg, "Welcome template disimpan.")
	return true
}

func (p *Plugin) handleSetGoodbyeTemplate(ctx *api.Context) bool {
	if !p.guardGroupAdmin(ctx) {
		return true
	}
	if len(ctx.Args) == 0 {
		current := strings.TrimSpace(p.bot.GetGroupConfig(ctx.Msg.Info.Chat).GoodbyeTemplate)
		if current == "" {
			p.bot.Reply(ctx.Msg, "Goodbye template kosong. Contoh: "+ctx.Prefix+"setgoodbye 👋 {user} keluar dari {group}")
			return true
		}
		p.bot.Reply(ctx.Msg, "Goodbye template:\n"+current)
		return true
	}
	tpl := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if strings.EqualFold(tpl, "reset") {
		tpl = ""
	}
	if err := p.bot.SetGroupFeature(ctx.Msg.Info.Chat, func(gc *api.GroupConfig) { gc.GoodbyeTemplate = tpl }); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan goodbye template")
		return true
	}
	if tpl == "" {
		p.bot.Reply(ctx.Msg, "Goodbye template direset ke default.")
		return true
	}
	p.bot.Reply(ctx.Msg, "Goodbye template disimpan.")
	return true
}
