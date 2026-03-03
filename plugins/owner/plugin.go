package owner

import "meow/plugins/api"

type Plugin struct {
	bot api.BotAPI
}

func New(bot api.BotAPI) *Plugin { return &Plugin{bot: bot} }

func (p *Plugin) Name() string { return "owner" }

func (p *Plugin) Commands() []string {
	return []string{
		"setowner", "addowner", "delowner", "listowner", "setprefix", "setwm", "setstickerwm", "autoread", "exec", "eval",
		"addlimit", "resetlimit", "dellimit", "setdaily",
		"owner/setowner", "owner/addowner", "owner/delowner", "owner/listowner",
		"owner/setprefix", "owner/setwm", "owner/setstickerwm", "owner/autoread", "owner/exec", "owner/eval",
		"owner/addlimit", "owner/resetlimit", "owner/dellimit", "owner/setdaily",
	}
}

func (p *Plugin) Handle(ctx *api.Context) bool {
	switch normalizeOwnerCommand(ctx.Command) {
	case "autoread":
		return p.handleAutoRead(ctx)
	case "setprefix":
		return p.handleSetPrefix(ctx)
	case "setwm", "setstickerwm":
		return p.handleSetWM(ctx)
	case "setowner":
		return p.handleSetOwner(ctx)
	case "addowner":
		return p.handleAddOwner(ctx)
	case "delowner":
		return p.handleDelOwner(ctx)
	case "listowner":
		return p.handleListOwner(ctx)
	case "addlimit":
		return p.handleAddLimit(ctx)
	case "resetlimit":
		return p.handleResetLimit(ctx)
	case "dellimit":
		return p.handleDelLimit(ctx)
	case "setdaily":
		return p.handleSetDaily(ctx)
	case "exec":
		return p.handleExec(ctx)
	case "eval":
		return p.handleEval(ctx)
	default:
		return false
	}
}
