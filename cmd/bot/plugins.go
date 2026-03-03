package main

import (
	"strings"

	"meow/plugins/api"
	plugindownloader "meow/plugins/downloader"
	plugingame "meow/plugins/game"
	plugingeneral "meow/plugins/general"
	plugingroup "meow/plugins/group"
	pluginmedia "meow/plugins/media"
	pluginowner "meow/plugins/owner"
)

func (b *Bot) registerDefaultPlugins() {
	b.registerPlugin(plugingeneral.New(b))
	b.registerPlugin(pluginowner.New(b))
	b.registerPlugin(plugingroup.New(b))
	b.registerPlugin(pluginmedia.New(b))
	b.registerPlugin(plugindownloader.New(b))
	b.registerPlugin(plugingame.New(b))
}

func (b *Bot) registerPlugin(plugin api.CommandPlugin) {
	for _, cmd := range plugin.Commands() {
		cmd = strings.ToLower(strings.TrimSpace(cmd))
		if cmd == "" {
			continue
		}
		b.commands[cmd] = plugin
	}
}

func (b *Bot) dispatchCommand(ctx *api.Context) bool {
	plugin, ok := b.commands[ctx.Command]
	if !ok {
		return false
	}
	return plugin.Handle(ctx)
}
