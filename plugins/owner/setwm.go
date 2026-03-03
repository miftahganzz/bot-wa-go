package owner

import (
	"fmt"
	"log"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleSetWM(ctx *api.Context) bool {
	if len(ctx.Args) == 0 {
		pack, author := p.bot.GetStickerWM()
		p.bot.Reply(ctx.Msg, fmt.Sprintf("WM sticker saat ini\nPack: %s\nAuthor: %s", pack, author))
		return true
	}
	if !p.bot.IsOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner")
		return true
	}

	raw := strings.TrimSpace(strings.Join(ctx.Args, " "))
	parts := strings.SplitN(raw, "|", 2)
	if len(parts) != 2 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"setwm meow bot|miftah")
		return true
	}
	pack := strings.TrimSpace(parts[0])
	author := strings.TrimSpace(parts[1])
	if pack == "" || author == "" {
		p.bot.Reply(ctx.Msg, "Pack dan author tidak boleh kosong")
		return true
	}
	if err := p.bot.SetStickerWM(pack, author); err != nil {
		p.bot.Reply(ctx.Msg, "Gagal simpan WM sticker")
		log.Printf("gagal simpan wm sticker: %v", err)
		return true
	}
	p.bot.Reply(ctx.Msg, fmt.Sprintf("WM sticker diupdate\nPack: %s\nAuthor: %s", pack, author))
	return true
}
