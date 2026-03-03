package media

import (
	"log"
	"strings"

	"meow/plugins/api"
)

type Plugin struct {
	bot api.BotAPI
}

func New(bot api.BotAPI) *Plugin { return &Plugin{bot: bot} }

func (p *Plugin) Name() string { return "media" }

func (p *Plugin) Commands() []string {
	return []string{"toimg", "sticker", "toaudio", "tovn", "qc", "brat", "upscale", "hd"}
}

func (p *Plugin) Handle(ctx *api.Context) bool {
	switch ctx.Command {
	case "toimg":
		if err := p.bot.HandleToImg(ctx.Msg); err != nil {
			log.Printf("toimg error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal toimg. Reply sticker/gambar lalu ketik .toimg")
		}
	case "sticker":
		if err := p.bot.HandleSticker(ctx.Msg); err != nil {
			log.Printf("sticker error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal sticker. Reply gambar/video ATAU kirim gambar/video dengan caption .sticker")
		}
	case "toaudio":
		if err := p.bot.HandleToAudio(ctx.Msg); err != nil {
			log.Printf("toaudio error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal toaudio. Reply video/audio ATAU kirim video/audio dengan caption .toaudio")
		}
	case "tovn":
		if err := p.bot.HandleToVN(ctx.Msg); err != nil {
			log.Printf("tovn error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal tovn. Reply video/audio ATAU kirim video/audio dengan caption .tovn")
		}
	case "qc":
		if err := p.bot.HandleQC(ctx.Msg, ctx.Args); err != nil {
			log.Printf("qc error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal qc. Reply pesan user lain pakai .qc (mode reply), atau kirim image+caption .qc <teks> (mode media)")
		}
	case "brat":
		if len(ctx.Args) == 0 {
			p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"brat halo dunia")
			return true
		}
		animate := false
		args := ctx.Args
		if len(args) > 0 {
			first := strings.ToLower(strings.TrimSpace(args[0]))
			if first == "-animate" || first == "--animate" {
				animate = true
				args = args[1:]
			}
		}
		text := strings.TrimSpace(strings.Join(args, " "))
		if text == "" {
			p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"brat -animate halo dunia")
			return true
		}
		var err error
		if animate {
			err = p.bot.HandleBratAnimate(ctx.Msg, text)
		} else {
			err = p.bot.HandleBrat(ctx.Msg, text)
		}
		if err != nil {
			log.Printf("brat error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal brat. Coba lagi sebentar.")
		}
	case "upscale", "hd":
		if err := p.bot.HandleUpscale(ctx.Msg, ctx.Args); err != nil {
			log.Printf("upscale error: %v", err)
			p.bot.Reply(ctx.Msg, "Gagal upscale. Reply gambar/sticker atau kirim gambar dengan caption .upscale [2|4]")
		}
	default:
		return false
	}
	return true
}
