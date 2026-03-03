package group

import "meow/plugins/api"

type Plugin struct {
	bot api.BotAPI
}

func New(bot api.BotAPI) *Plugin { return &Plugin{bot: bot} }

func (p *Plugin) Name() string { return "group" }

func (p *Plugin) Commands() []string {
	return []string{
		"antilink", "welcome", "goodbye", "tagall", "hidetag", "setwelcome", "setgoodbye",
		"group/antilink", "group/welcome", "group/goodbye", "group/tagall", "group/hidetag", "group/setwelcome", "group/setgoodbye",
	}
}

func (p *Plugin) Handle(ctx *api.Context) bool {
	switch normalizeGroupCommand(ctx.Command) {
	case "antilink":
		return p.handleAntiLink(ctx)
	case "welcome":
		return p.handleWelcome(ctx)
	case "goodbye":
		return p.handleGoodbye(ctx)
	case "tagall":
		return p.handleTagAll(ctx)
	case "hidetag":
		return p.handleHideTag(ctx)
	case "setwelcome":
		return p.handleSetWelcomeTemplate(ctx)
	case "setgoodbye":
		return p.handleSetGoodbyeTemplate(ctx)
	default:
		return false
	}
}
