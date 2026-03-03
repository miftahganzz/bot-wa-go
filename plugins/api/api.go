package api

import (
	"time"

	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
)

type GroupConfig struct {
	AntiLink        bool   `json:"antilink"`
	Welcome         bool   `json:"welcome"`
	Goodbye         bool   `json:"goodbye"`
	WelcomeTemplate string `json:"welcome_template"`
	GoodbyeTemplate string `json:"goodbye_template"`
}

type Context struct {
	Msg     *events.Message
	Prefix  string
	Command string
	Args    []string
	Text    string
	Started time.Time
}

type CommandPlugin interface {
	Name() string
	Commands() []string
	Handle(ctx *Context) bool
}

type BotAPI interface {
	Reply(msg *events.Message, text string)
	ReplyMention(msg *events.Message, text string, mentions []string)
	SendMenuButtons(msg *events.Message) error
	HelpText() string
	PingText(msg *events.Message, started time.Time) string
	RuntimeText() string

	GetPrefixes() []string
	GetAutoRead() bool
	SetAutoRead(bool) error
	GetStickerWM() (string, string)
	SetStickerWM(pack, author string) error

	IsOwner(msg *events.Message) bool
	IsSuperOwner(msg *events.Message) bool
	SetPrefixes([]string) error
	SetOwner(msg *events.Message, owner string) error
	AddOwner(msg *events.Message, owner string) error
	DelOwner(msg *events.Message, owner string) error
	ListOwners() (super []string, extra []string)

	ParseOnOff(s string) (bool, bool)
	ParsePrefixArgs(args []string) ([]string, error)
	NormalizePhone(s string) string

	IsGroupAdmin(msg *events.Message) bool
	GetGroupConfig(chat types.JID) GroupConfig
	SetGroupFeature(chat types.JID, update func(*GroupConfig)) error
	SendTagAll(msg *events.Message, extra string, hidden bool) error

	HandleToImg(msg *events.Message) error
	HandleSticker(msg *events.Message) error
	HandleToAudio(msg *events.Message) error
	HandleToVN(msg *events.Message) error
	HandleQC(msg *events.Message, args []string) error
	HandleBrat(msg *events.Message, text string) error
	HandleBratAnimate(msg *events.Message, text string) error
	HandleUpscale(msg *events.Message, args []string) error
	HandleTikTok(msg *events.Message, inputURL string) error
	HandleInstagram(msg *events.Message, inputURL string) error
	HandleProfile(msg *events.Message, args []string) error
	HandleLeaderboard(msg *events.Message, args []string) error
	HandleDaily(msg *events.Message) error
	HandleBalance(msg *events.Message, args []string) error
	HandleWork(msg *events.Message) error
	HandleTransfer(msg *events.Message, args []string) error
	HandleBuyLimit(msg *events.Message, args []string) error
	HandleAFK(msg *events.Message, args []string) error
	HandleTicket(msg *events.Message, args []string) error
	HandleTebakBendera(msg *events.Message, args []string) error
	HandleTebakKartun(msg *events.Message, args []string) error
	HandleTebakKimia(msg *events.Message, args []string) error
	HandleTebakGambar(msg *events.Message, args []string) error
	HandleMathGame(msg *events.Message, args []string) error

	RunExec(command string) (string, error)
	EvalExpression(expr string) (string, error)

	OwnerAddLimit(msg *events.Message, phone string, amount int) (int, error)
	OwnerResetLimit(msg *events.Message, phone string) (int, error)
	OwnerDelLimit(msg *events.Message, phone string) (int, error)
	OwnerSetDaily(msg *events.Message, xp int64, limit, cooldownHours int) error
	GetDailyConfig() (int64, int, int)
	ResolveTargetPhone(msg *events.Message, raw string) (string, error)
}
