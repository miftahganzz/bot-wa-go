package general

import (
	"log"
	"strings"

	"meow/plugins/api"
)

type Plugin struct {
	bot api.BotAPI
}

func New(bot api.BotAPI) *Plugin { return &Plugin{bot: bot} }

func (p *Plugin) Name() string { return "general" }

func (p *Plugin) Commands() []string {
	return []string{
		"ping", "help", "menu", "mmenu", "runtime", "uptime", "prefix", "prefixes",
		"profile", "me", "leaderboard", "lb", "daily", "claim",
		"balance", "wallet", "work", "transfer", "buylimit", "afk", "ticket",
	}
}

func (p *Plugin) Handle(ctx *api.Context) bool {
	switch ctx.Command {
	case "ping":
		p.bot.Reply(ctx.Msg, p.bot.PingText(ctx.Msg, ctx.Started))
	case "help", "menu", "mmenu":
		if err := p.bot.SendMenuButtons(ctx.Msg); err != nil {
			log.Printf("gagal kirim menu buttons: %v", err)
			p.bot.Reply(ctx.Msg, p.bot.HelpText())
		}
	case "runtime", "uptime":
		p.bot.Reply(ctx.Msg, p.bot.RuntimeText())
	case "prefix", "prefixes":
		p.bot.Reply(ctx.Msg, "Prefix aktif: "+strings.Join(p.bot.GetPrefixes(), " "))
	case "profile", "me":
		if err := p.bot.HandleProfile(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "profile error: "+err.Error())
		}
	case "leaderboard", "lb":
		if err := p.bot.HandleLeaderboard(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "leaderboard error: "+err.Error())
		}
	case "daily", "claim":
		if err := p.bot.HandleDaily(ctx.Msg); err != nil {
			p.bot.Reply(ctx.Msg, "daily error: "+err.Error())
		}
	case "balance", "wallet":
		if err := p.bot.HandleBalance(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "balance error: "+err.Error())
		}
	case "work":
		if err := p.bot.HandleWork(ctx.Msg); err != nil {
			p.bot.Reply(ctx.Msg, "work error: "+err.Error())
		}
	case "transfer":
		if err := p.bot.HandleTransfer(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "transfer error: "+err.Error())
		}
	case "buylimit":
		if err := p.bot.HandleBuyLimit(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "buylimit error: "+err.Error())
		}
	case "afk":
		if err := p.bot.HandleAFK(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "afk error: "+err.Error())
		}
	case "ticket":
		if err := p.bot.HandleTicket(ctx.Msg, ctx.Args); err != nil {
			p.bot.Reply(ctx.Msg, "ticket error: "+err.Error())
		}
	default:
		return false
	}
	return true
}
