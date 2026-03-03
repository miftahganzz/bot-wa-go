package owner

import (
	"fmt"
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleDelOwner(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Hanya owner utama (config.json) yang bisa delowner")
		return true
	}
	if len(ctx.Args) != 1 {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: "+ctx.Prefix+"delowner 62812xxxxxx")
		return true
	}
	owner := p.bot.NormalizePhone(ctx.Args[0])
	if owner == "" {
		p.bot.Reply(ctx.Msg, "Nomor owner tidak valid")
		return true
	}
	if err := p.bot.DelOwner(ctx.Msg, owner); err != nil {
		p.bot.Reply(ctx.Msg, err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, "Owner tambahan dihapus: "+owner)
	return true
}

func (p *Plugin) handleListOwner(ctx *api.Context) bool {
	if !p.bot.IsOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner")
		return true
	}
	super, extra := p.bot.ListOwners()
	mentions := make([]string, 0, len(super)+len(extra))

	var sb strings.Builder
	sb.WriteString("Daftar owner\n\nOwner utama (full access):\n")
	if len(super) == 0 {
		sb.WriteString("- (kosong)\n")
	} else {
		for i, n := range super {
			sb.WriteString("- @")
			sb.WriteString(n)
			sb.WriteString(" (utama ")
			sb.WriteString(fmt.Sprintf("%d", i+1))
			sb.WriteString(")")
			sb.WriteString("\n")
			mentions = append(mentions, n+"@s.whatsapp.net")
		}
	}

	sb.WriteString("\nOwner tambahan:\n")
	if len(extra) == 0 {
		sb.WriteString("- (kosong)\n")
	} else {
		for _, n := range extra {
			sb.WriteString("- @")
			sb.WriteString(n)
			sb.WriteString("\n")
			mentions = append(mentions, n+"@s.whatsapp.net")
		}
	}
	sb.WriteString("\nCatatan: owner tambahan tidak bisa pakai $ dan x.")
	p.bot.ReplyMention(ctx.Msg, strings.TrimSpace(sb.String()), mentions)
	return true
}
