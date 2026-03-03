package owner

import (
	"strings"

	"meow/plugins/api"
)

func (p *Plugin) handleExec(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner utama")
		return true
	}
	command := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if command == "" {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: $ ls -la")
		return true
	}
	out, err := p.bot.RunExec(command)
	if err != nil {
		if strings.TrimSpace(out) == "" {
			p.bot.Reply(ctx.Msg, "exec error: "+err.Error())
			return true
		}
		p.bot.Reply(ctx.Msg, "exec error: "+err.Error()+"\n\n"+out)
		return true
	}
	if strings.TrimSpace(out) == "" {
		p.bot.Reply(ctx.Msg, "exec berhasil (tanpa output)")
		return true
	}
	p.bot.Reply(ctx.Msg, out)
	return true
}

func (p *Plugin) handleEval(ctx *api.Context) bool {
	if !p.bot.IsSuperOwner(ctx.Msg) {
		p.bot.Reply(ctx.Msg, "Command ini khusus owner utama")
		return true
	}
	expr := strings.TrimSpace(strings.Join(ctx.Args, " "))
	if expr == "" {
		p.bot.Reply(ctx.Msg, "Format salah. Contoh: x (2+3)*10")
		return true
	}
	result, err := p.bot.EvalExpression(expr)
	if err != nil {
		p.bot.Reply(ctx.Msg, "eval error: "+err.Error())
		return true
	}
	p.bot.Reply(ctx.Msg, "=> "+result)
	return true
}
