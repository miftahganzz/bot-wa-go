package game

import (
	"log"

	"meow/plugins/api"
)

type Plugin struct {
	bot api.BotAPI
}

func New(bot api.BotAPI) *Plugin { return &Plugin{bot: bot} }

func (p *Plugin) Name() string { return "game" }

func (p *Plugin) Commands() []string {
	return []string{
		"tebakbendera", "tb", "bendera",
		"tebakkartun", "tk", "kartun",
		"tebakkimia", "tkimia", "kimia",
		"tebakgambar", "tg", "gambar",
		"math", "maths", "mtk",
		"game/tebakbendera", "game/tb", "game/bendera",
		"game/tebakkartun", "game/tk", "game/kartun",
		"game/tebakkimia", "game/tkimia", "game/kimia",
		"game/tebakgambar", "game/tg", "game/gambar",
		"game/math", "game/maths", "game/mtk",
	}
}

func (p *Plugin) Handle(ctx *api.Context) bool {
	switch normalizeGameCommand(ctx.Command) {
	case "tebakbendera", "tb", "bendera":
		if err := p.bot.HandleTebakBendera(ctx.Msg, ctx.Args); err != nil {
			log.Printf("tebakbendera error: %v", err)
			p.bot.Reply(ctx.Msg, "Game error: "+err.Error())
		}
		return true
	case "tebakkartun", "tk", "kartun":
		if err := p.bot.HandleTebakKartun(ctx.Msg, ctx.Args); err != nil {
			log.Printf("tebakkartun error: %v", err)
			p.bot.Reply(ctx.Msg, "Game error: "+err.Error())
		}
		return true
	case "tebakkimia", "tkimia", "kimia":
		if err := p.bot.HandleTebakKimia(ctx.Msg, ctx.Args); err != nil {
			log.Printf("tebakkimia error: %v", err)
			p.bot.Reply(ctx.Msg, "Game error: "+err.Error())
		}
		return true
	case "tebakgambar", "tg", "gambar":
		if err := p.bot.HandleTebakGambar(ctx.Msg, ctx.Args); err != nil {
			log.Printf("tebakgambar error: %v", err)
			p.bot.Reply(ctx.Msg, "Game error: "+err.Error())
		}
		return true
	case "math", "maths", "mtk":
		if err := p.bot.HandleMathGame(ctx.Msg, ctx.Args); err != nil {
			log.Printf("math game error: %v", err)
			p.bot.Reply(ctx.Msg, "Math error: "+err.Error())
		}
		return true
	default:
		return false
	}
}

func normalizeGameCommand(cmd string) string {
	if len(cmd) > 5 && cmd[:5] == "game/" {
		return cmd[5:]
	}
	return cmd
}
