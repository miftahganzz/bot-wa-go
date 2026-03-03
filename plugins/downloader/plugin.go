package downloader

import (
	"log"
	"strings"

	"meow/plugins/api"
)

type Plugin struct {
	bot api.BotAPI
}

func New(bot api.BotAPI) *Plugin { return &Plugin{bot: bot} }

func (p *Plugin) Name() string { return "downloader" }

func (p *Plugin) Commands() []string {
	return []string{"tiktok", "tt", "instagram", "ig"}
}

func (p *Plugin) Handle(ctx *api.Context) bool {
	switch ctx.Command {
	case "tiktok", "tt":
		if len(ctx.Args) == 0 {
			p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"tiktok https://www.tiktok.com/@user/video/123")
			return true
		}
		targetURL := extractURLArg(ctx.Args)
		if targetURL == "" {
			p.bot.Reply(ctx.Msg, "URL TikTok tidak ditemukan")
			return true
		}
		if err := p.bot.HandleTikTok(ctx.Msg, targetURL); err != nil {
			log.Printf("tiktok error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal ambil TikTok. Pastikan link valid/public.")
		}
	case "instagram", "ig":
		if len(ctx.Args) == 0 {
			p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"ig https://www.instagram.com/reel/xxxx/")
			return true
		}
		targetURL := extractURLArg(ctx.Args)
		if targetURL == "" {
			p.bot.Reply(ctx.Msg, "URL Instagram tidak ditemukan")
			return true
		}
		if err := p.bot.HandleInstagram(ctx.Msg, targetURL); err != nil {
			log.Printf("instagram error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal ambil Instagram. Cek link valid/public.")
		}
	default:
		return false
	}
	return true
}

func extractURLArg(args []string) string {
	for _, part := range args {
		p := strings.TrimSpace(part)
		if strings.HasPrefix(p, "http://") || strings.HasPrefix(p, "https://") {
			return p
		}
	}
	return ""
}
