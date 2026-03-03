package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"log"
	"math"
	"math/rand"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode"

	"meow/plugins/api"

	_ "github.com/mattn/go-sqlite3"
	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	waLog "go.mau.fi/whatsmeow/util/log"
	"google.golang.org/protobuf/proto"
)

const (
	defaultDBPath      = "file:session.db?_foreign_keys=on"
	welcomeCardAPI     = "https://api.siputzx.my.id/api/canvas/welcomev5"
	goodbyeCardAPI     = "https://api.siputzx.my.id/api/canvas/goodbyev5"
	profileCardAPI     = "https://api.siputzx.my.id/api/canvas/profile"
	levelUpCardAPI     = "https://api.siputzx.my.id/api/canvas/level-up"
	defaultCardBG      = "https://i.ibb.co/4YBNyvP/mountain-sunset.jpg"
	defaultAvatarURL   = "https://avatars.githubusercontent.com/u/159487561?v=4"
	bootstrapMark      = ".meow_bootstrap_done"
	workCooldown       = 60 * time.Minute
	workMinCoin        = int64(80)
	workMaxCoin        = int64(220)
	limitPriceCoin     = int64(10)
	transferCooldown   = 30 * time.Second
	transferDailyMax   = int64(5000)
	flagQuizTimeout    = 2 * time.Minute
	flagQuizReward     = int64(150)
	cartoonQuizTimeout = 2 * time.Minute
	cartoonQuizReward  = int64(180)
	kimiaQuizTimeout   = 2 * time.Minute
	kimiaQuizReward    = int64(140)
	gambarQuizTimeout  = 2 * time.Minute
	gambarQuizReward   = int64(170)
)

var defaultPrefixes = []string{".", "!", "/", "?", "#"}
var linkRegex = regexp.MustCompile(`(?i)(https?://|chat\.whatsapp\.com/|wa\.me/|t\.me/|discord\.gg/)`)
var ignoredClientErrorSubstrings = []string{
	"Failed to handle retry receipt",
	"couldn't find message",
}

const (
	clrReset  = "\033[0m"
	clrBold   = "\033[1m"
	clrCyan   = "\033[36m"
	clrBlue   = "\033[34m"
	clrGreen  = "\033[32m"
	clrYellow = "\033[33m"
	clrRed    = "\033[31m"
)

type Bot struct {
	client      *whatsmeow.Client
	store       *MongoStore
	superOwners []string
	startedAt   time.Time
	commands    map[string]api.CommandPlugin
	xpMu        sync.Mutex
	xpCooldown  map[string]time.Time
	flagMu      sync.Mutex
	flagQuiz    map[string]flagQuizState
	cartoonMu   sync.Mutex
	cartoonQuiz map[string]cartoonQuizState
	kimiaMu     sync.Mutex
	kimiaQuiz   map[string]kimiaQuizState
	gambarMu    sync.Mutex
	gambarQuiz  map[string]gambarQuizState
	mathMu      sync.Mutex
	mathQuiz    map[string]mathQuizState
}

type flagQuizState struct {
	Answer    string
	StartedAt time.Time
	ImageURL  string
}

type cartoonQuizState struct {
	Answer    string
	StartedAt time.Time
	ImageURL  string
}

type kimiaQuizState struct {
	Answer    string
	Question  string
	StartedAt time.Time
}

type gambarQuizState struct {
	Answer      string
	ImageURL    string
	Description string
	StartedAt   time.Time
}

type mathQuizState struct {
	Answer    int64
	Mode      string
	Bonus     int64
	ExpiresAt time.Time
	Question  string
}

type filteredLogger struct {
	base waLog.Logger
}

func newFilteredLogger(base waLog.Logger) waLog.Logger {
	return &filteredLogger{base: base}
}

func (l *filteredLogger) Warnf(msg string, args ...interface{}) {
	l.base.Warnf(msg, args...)
}

func (l *filteredLogger) Errorf(msg string, args ...interface{}) {
	for _, skip := range ignoredClientErrorSubstrings {
		if strings.Contains(msg, skip) {
			return
		}
	}
	l.base.Errorf(msg, args...)
}

func (l *filteredLogger) Infof(msg string, args ...interface{}) {
	l.base.Infof(msg, args...)
}

func (l *filteredLogger) Debugf(msg string, args ...interface{}) {
	l.base.Debugf(msg, args...)
}

func (l *filteredLogger) Sub(module string) waLog.Logger {
	return &filteredLogger{base: l.base.Sub(module)}
}

func main() {
	ctx := context.Background()
	log.SetFlags(0)
	rand.Seed(time.Now().UnixNano())

	authMode := flag.String("auth", "", "mode login: qr, pair, atau both")
	pairPhone := flag.String("pair-phone", "", "nomor HP untuk pairing code (contoh: 62812xxxxxx)")
	owner := flag.String("owner", "", "nomor owner (contoh: 62812xxxxxx)")
	flag.Parse()

	printBanner()
	runBootstrap()

	cfg, err := loadBotConfig(defaultBotCfg)
	if err != nil {
		log.Fatalf("gagal baca config.json: %v", err)
	}

	cfgChanged := false
	if o := normalizePhone(*owner); o != "" {
		cfg.Owner = o
		cfg.OwnerNumbers = normalizePhoneList([]string{o})
		cfgChanged = true
	}
	if len(cfg.OwnerNumbers) == 0 && cfg.Owner != "" {
		cfg.OwnerNumbers = normalizePhoneList([]string{cfg.Owner})
		cfgChanged = true
	}
	for len(cfg.OwnerNumbers) == 0 {
		input, err := promptLine(prettyPrompt("Masukkan nomor owner utama (wajib, contoh 62812xxxxxx): "))
		if err != nil {
			log.Fatalf("gagal baca input owner: %v", err)
		}
		ownerNumber := normalizePhone(input)
		if ownerNumber == "" {
			printWarn("nomor owner tidak valid, coba lagi")
		} else {
			cfg.OwnerNumbers = []string{ownerNumber}
			cfg.Owner = ownerNumber
			cfgChanged = true
		}
	}

	if p := normalizePhone(*pairPhone); p != "" {
		cfg.BotNumber = p
		cfgChanged = true
	}

	if cfgChanged {
		if err := saveBotConfigMerged(defaultBotCfg, cfg); err != nil {
			log.Fatalf("gagal simpan config.json: %v", err)
		}
	}

	printStep("Init", "mempersiapkan database session")
	dbLog := waLog.Stdout("Database", "INFO", true)
	container, err := sqlstore.New(ctx, "sqlite3", defaultDBPath, dbLog)
	if err != nil {
		log.Fatalf("gagal inisialisasi database: %v", err)
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		log.Fatalf("gagal ambil device store: %v", err)
	}

	printStep("Init", "mempersiapkan WhatsApp client")
	clientLog := newFilteredLogger(waLog.Stdout("Client", "INFO", true))
	client := whatsmeow.NewClient(deviceStore, clientLog)

	bot, err := NewBot(client, cfg.OwnerNumbers)
	if err != nil {
		log.Fatalf("gagal inisialisasi config bot: %v", err)
	}

	client.AddEventHandler(func(evt interface{}) {
		switch v := evt.(type) {
		case *events.Message:
			bot.HandleIncomingMessage(v)
		case *events.GroupInfo:
			bot.HandleGroupInfoEvent(v)
		}
	})

	resolvedAuth, resolvedPairPhone, err := resolveLoginOptions(client.Store.ID != nil, *authMode, *pairPhone, cfg.BotNumber)
	if err != nil {
		log.Fatalf("gagal baca opsi login: %v", err)
	}
	if resolvedPairPhone != "" && cfg.BotNumber != resolvedPairPhone {
		cfg.BotNumber = resolvedPairPhone
		if err := saveBotConfigMerged(defaultBotCfg, cfg); err != nil {
			printWarn("gagal simpan bot_number ke config.json: " + err.Error())
		}
	}

	printStep("Login", "menghubungkan ke WhatsApp")
	if err := connectClient(client, resolvedAuth, resolvedPairPhone); err != nil {
		log.Fatalf("gagal login/connect WhatsApp: %v", err)
	}

	printOK("bot aktif, tekan Ctrl+C untuk keluar")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	if err := bot.store.Close(context.Background()); err != nil {
		printWarn("gagal close mongo: " + err.Error())
	}
	client.Disconnect()
	printStep("Shutdown", "bot berhenti")
}

func resolveLoginOptions(hasSession bool, authMode, pairPhone, cfgBotNumber string) (string, string, error) {
	if hasSession {
		return "", "", nil
	}

	authMode = strings.ToLower(strings.TrimSpace(authMode))
	pairPhone = normalizePhone(pairPhone)
	cfgBotNumber = normalizePhone(cfgBotNumber)

	if authMode == "" {
		fmt.Println()
		fmt.Printf("%s%sPilih Metode Login%s\n", clrBold, clrCyan, clrReset)
		fmt.Printf("%s1.%s QR Code\n", clrBlue, clrReset)
		fmt.Printf("%s2.%s Pairing Code (nomor)\n", clrBlue, clrReset)
		input, err := promptLine(prettyPrompt("Pilih opsi [1/2] (default: 2): "))
		if err != nil {
			return "", "", err
		}
		switch strings.TrimSpace(strings.ToLower(input)) {
		case "1", "qr":
			authMode = "qr"
		case "", "2", "pair":
			authMode = "pair"
		default:
			return "", "", fmt.Errorf("opsi login tidak valid: %q", input)
		}
	}

	if authMode != "qr" && authMode != "pair" && authMode != "both" {
		return "", "", fmt.Errorf("auth tidak valid: %q (pakai: qr | pair | both)", authMode)
	}

	if (authMode == "pair" || authMode == "both") && pairPhone == "" {
		pairPhone = cfgBotNumber
		if pairPhone == "" {
			input, err := promptLine(prettyPrompt("Masukkan nomor bot untuk pairing (contoh 17789019991): "))
			if err != nil {
				return "", "", err
			}
			pairPhone = normalizePhone(input)
		}
	}

	return authMode, pairPhone, nil
}

func promptLine(label string) (string, error) {
	fmt.Print(label)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func prettyPrompt(label string) string {
	return clrBold + clrBlue + "➤ " + clrReset + label
}

func connectClient(client *whatsmeow.Client, authMode, pairPhone string) error {
	if client.Store.ID != nil {
		return client.Connect()
	}

	if authMode == "" {
		authMode = "both"
	}
	if authMode != "qr" && authMode != "pair" && authMode != "both" {
		return fmt.Errorf("auth tidak valid: %q (pakai: qr | pair | both)", authMode)
	}

	normalizedPhone := normalizePhone(pairPhone)
	if (authMode == "pair" || authMode == "both") && normalizedPhone == "" {
		return errors.New("mode pair/both butuh --pair-phone")
	}

	qrChan, err := client.GetQRChannel(context.Background())
	if err != nil {
		return fmt.Errorf("gagal buat channel QR: %w", err)
	}

	if err := client.Connect(); err != nil {
		return fmt.Errorf("gagal connect ke WhatsApp: %w", err)
	}

	pairRequested := false
	for evt := range qrChan {
		switch evt.Event {
		case "code":
			if authMode == "qr" || authMode == "both" {
				fmt.Printf("\n%s%sQR Login%s\n", clrBold, clrCyan, clrReset)
				fmt.Printf("Scan QR ini dari WhatsApp > Linked Devices:\n%s\n\n", evt.Code)
			}

			if (authMode == "pair" || authMode == "both") && !pairRequested {
				code, err := client.PairPhone(context.Background(), normalizedPhone, true, whatsmeow.PairClientChrome, "Chrome (macOS)")
				if err != nil {
					return fmt.Errorf("gagal generate pairing code: %w", err)
				}
				printOK("pairing code: " + code)
				printStep("Pairing", "masukkan code di WhatsApp > Linked Devices > Link with phone number")
				pairRequested = true
			}
		case "success":
			printOK("login berhasil")
			return nil
		case "timeout":
			return errors.New("login timeout, jalankan ulang program")
		case "error":
			if evt.Error != nil {
				return fmt.Errorf("error login: %w", evt.Error)
			}
			return errors.New("error login tanpa detail")
		}
	}

	return errors.New("channel login tertutup sebelum sukses")
}

func NewBot(client *whatsmeow.Client, superOwners []string) (*Bot, error) {
	store, err := NewMongoStore()
	if err != nil {
		return nil, err
	}
	superOwners = normalizePhoneList(superOwners)
	bot := &Bot{
		client:      client,
		store:       store,
		superOwners: superOwners,
		startedAt:   time.Now(),
		commands:    make(map[string]api.CommandPlugin),
		xpCooldown:  make(map[string]time.Time),
		flagQuiz:    make(map[string]flagQuizState),
		cartoonQuiz: make(map[string]cartoonQuizState),
		kimiaQuiz:   make(map[string]kimiaQuizState),
		gambarQuiz:  make(map[string]gambarQuizState),
		mathQuiz:    make(map[string]mathQuizState),
	}

	if len(bot.superOwners) == 0 {
		legacyOwner := bot.store.Owner()
		if legacyOwner != "" {
			bot.superOwners = []string{legacyOwner}
		}
	}
	if len(bot.superOwners) == 0 {
		return nil, fmt.Errorf("owner utama wajib di config.json (field owner_number)")
	}
	// Keep first owner in global settings as backward-compat info.
	if err := bot.store.SetOwner(bot.superOwners[0]); err != nil {
		return nil, err
	}

	bot.registerDefaultPlugins()
	return bot, nil
}

func printBanner() {
	fmt.Printf("%s%s\n", clrCyan, "╔══════════════════════════════════════╗")
	fmt.Printf("║             MEOW BOT GO             ║\n")
	fmt.Printf("╚══════════════════════════════════════╝%s\n", clrReset)
}

func printStep(title, msg string) {
	fmt.Printf("%s[%s]%s %s\n", clrCyan, title, clrReset, msg)
}

func printOK(msg string) {
	fmt.Printf("%s[OK]%s %s\n", clrGreen, clrReset, msg)
}

func printWarn(msg string) {
	fmt.Printf("%s[WARN]%s %s\n", clrYellow, clrReset, msg)
}

func runBootstrap() {
	if _, err := os.Stat(bootstrapMark); err == nil {
		return
	}
	printStep("Bootstrap", "first run detected, menyiapkan dependency Go")
	if _, err := exec.LookPath("go"); err != nil {
		printWarn("binary go tidak ditemukan, skip bootstrap")
		return
	}
	steps := []struct {
		name string
		args []string
	}{
		{name: "download module", args: []string{"mod", "download"}},
		{name: "build project", args: []string{"build", "./..."}},
	}
	for _, s := range steps {
		printStep("Bootstrap", s.name)
		cmd := exec.Command("go", s.args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			printWarn(fmt.Sprintf("bootstrap step gagal (%s): %v", s.name, err))
			if text := strings.TrimSpace(string(out)); text != "" {
				printWarn(text)
			}
			return
		}
	}
	if err := os.WriteFile(bootstrapMark, []byte(time.Now().Format(time.RFC3339)+"\n"), 0o644); err != nil {
		printWarn("gagal tulis marker bootstrap: " + err.Error())
		return
	}
	printOK("bootstrap selesai")
}

func saveBotConfigMerged(path string, cfg BotConfig) error {
	raw := map[string]any{}
	if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
		_ = json.Unmarshal(b, &raw)
	}
	cfg.OwnerNumbers = normalizePhoneList(cfg.OwnerNumbers)
	if len(cfg.OwnerNumbers) > 0 {
		cfg.Owner = cfg.OwnerNumbers[0]
	}
	raw["mongo_uri"] = cfg.MongoURI
	raw["mongo_db"] = cfg.MongoDB
	raw["owner"] = normalizePhone(cfg.Owner)
	raw["owner_number"] = cfg.OwnerNumbers
	raw["bot_number"] = normalizePhone(cfg.BotNumber)
	return saveRawBotConfig(path, raw)
}

func (b *Bot) HandleIncomingMessage(msg *events.Message) {
	if msg.Info.IsFromMe || msg.Info.Timestamp.IsZero() {
		return
	}
	b.trackUser(msg)
	b.handleAFKOnIncoming(msg)
	b.maybeGainXP(msg)
	b.maybeMarkRead(msg)

	text := strings.TrimSpace(extractText(msg))
	if text == "" {
		return
	}

	if b.handleAntiLink(msg, text) {
		return
	}
	started := time.Now()

	if b.handleOwnerShortcuts(msg, text, started) {
		return
	}

	prefix, body, ok := b.extractCommand(text)
	if !ok {
		return
	}

	parts := strings.Fields(body)
	if len(parts) == 0 {
		return
	}

	cmd := strings.ToLower(parts[0])
	args := parts[1:]
	if cost := b.commandLimitCost(cmd, args); cost > 0 {
		if err := b.consumeLimitForCommand(msg, cost); err != nil {
			b.reply(msg, err.Error())
			return
		}
	}
	ctx := &api.Context{
		Msg:     msg,
		Prefix:  prefix,
		Command: cmd,
		Args:    args,
		Text:    text,
		Started: started,
	}
	if !b.dispatchCommand(ctx) {
		b.reply(msg, "Command tidak dikenal. Pakai "+prefix+"help")
	}
}

func (b *Bot) maybeGainXP(msg *events.Message) {
	if msg == nil {
		return
	}
	jid := msg.Info.Sender.ToNonAD()
	if jid.User == "" {
		jid = msg.Info.Chat.ToNonAD()
	}
	if jid.User == "" {
		return
	}
	key := jid.String()
	now := time.Now()

	b.xpMu.Lock()
	last := b.xpCooldown[key]
	if !last.IsZero() && now.Sub(last) < 20*time.Second {
		b.xpMu.Unlock()
		return
	}
	b.xpCooldown[key] = now
	b.xpMu.Unlock()

	gain := int64(8 + rand.Intn(8))
	stats, err := b.store.AddUserXP(key, gain)
	if err != nil {
		log.Printf("gagal add xp: %v", err)
		return
	}
	fromLevel := stats.Level
	if fromLevel <= 0 {
		fromLevel = 1
	}
	toLevel := levelFromXP(stats.XP)
	if toLevel <= fromLevel {
		return
	}
	if err := b.store.SetUserLevel(key, toLevel); err != nil {
		log.Printf("gagal set level user: %v", err)
		return
	}
	if err := b.sendLevelUpCard(msg, fromLevel, toLevel, stats); err != nil {
		log.Printf("gagal kirim level up card: %v", err)
	}
}

func levelFromXP(xp int64) int {
	if xp <= 0 {
		return 1
	}
	level := int(math.Sqrt(float64(xp)/100.0)) + 1
	if level < 1 {
		return 1
	}
	return level
}

func baseXPForLevel(level int) int64 {
	if level <= 1 {
		return 0
	}
	l := int64(level - 1)
	return l * l * 100
}

func expProgress(xp int64, level int) (int64, int64) {
	base := baseXPForLevel(level)
	nextBase := baseXPForLevel(level + 1)
	progress := xp - base
	if progress < 0 {
		progress = 0
	}
	need := nextBase - base
	if need <= 0 {
		need = 100
	}
	return progress, need
}

func rankNameByLevel(level int) string {
	switch {
	case level >= 60:
		return "mythic"
	case level >= 40:
		return "legend"
	case level >= 25:
		return "master"
	case level >= 15:
		return "epic"
	case level >= 8:
		return "elite"
	default:
		return "beginner"
	}
}

func (b *Bot) handleOwnerShortcuts(msg *events.Message, text string, started time.Time) bool {
	if !b.isSuperOwner(msg) {
		return false
	}
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	if strings.HasPrefix(trimmed, "$") {
		body := strings.TrimSpace(strings.TrimPrefix(trimmed, "$"))
		if body == "" {
			b.reply(msg, "Format salah. Contoh: $ ls -la")
			return true
		}
		ctx := &api.Context{
			Msg:     msg,
			Prefix:  "$",
			Command: "exec",
			Args:    strings.Fields(body),
			Text:    text,
			Started: started,
		}
		return b.dispatchCommand(ctx)
	}

	if strings.HasPrefix(strings.ToLower(trimmed), "x ") || strings.EqualFold(trimmed, "x") {
		body := strings.TrimSpace(trimmed[1:])
		if body == "" {
			b.reply(msg, "Format salah. Contoh: x (2+3)*10")
			return true
		}
		ctx := &api.Context{
			Msg:     msg,
			Prefix:  "x",
			Command: "eval",
			Args:    strings.Fields(body),
			Text:    text,
			Started: started,
		}
		return b.dispatchCommand(ctx)
	}
	return false
}

func (b *Bot) HandleGroupInfoEvent(evt *events.GroupInfo) {
	if evt == nil {
		return
	}
	gc := b.getGroupConfig(evt.JID)
	if len(evt.Join) > 0 && gc.Welcome {
		parts := make([]string, 0, len(evt.Join))
		mentions := make([]string, 0, len(evt.Join))
		for _, j := range evt.Join {
			jid := j.ToNonAD().String()
			mentions = append(mentions, jid)
			parts = append(parts, "@"+j.User)
		}
		text := b.renderGroupTemplate(gc.WelcomeTemplate, true, strings.Join(parts, ", "), b.groupName(evt.JID), len(evt.Join))
		if text == "" {
			text = "✨ Welcome " + strings.Join(parts, ", ") + "\nKe grup: " + b.groupName(evt.JID)
		}
		if err := b.sendGroupEventCard(evt.JID, evt.Join[0].ToNonAD(), len(evt.Join), true, text, mentions); err != nil {
			log.Printf("gagal kirim welcome card: %v", err)
			b.sendMentionMessage(evt.JID, text, mentions)
		}
	}
	if len(evt.Leave) > 0 && gc.Goodbye {
		parts := make([]string, 0, len(evt.Leave))
		mentions := make([]string, 0, len(evt.Leave))
		for _, j := range evt.Leave {
			jid := j.ToNonAD().String()
			mentions = append(mentions, jid)
			parts = append(parts, "@"+j.User)
		}
		text := b.renderGroupTemplate(gc.GoodbyeTemplate, false, strings.Join(parts, ", "), b.groupName(evt.JID), len(evt.Leave))
		if text == "" {
			text = "👋 Goodbye " + strings.Join(parts, ", ") + "\nDari grup: " + b.groupName(evt.JID)
		}
		if err := b.sendGroupEventCard(evt.JID, evt.Leave[0].ToNonAD(), len(evt.Leave), false, text, mentions); err != nil {
			log.Printf("gagal kirim goodbye card: %v", err)
			b.sendMentionMessage(evt.JID, text, mentions)
		}
	}
}

func (b *Bot) renderGroupTemplate(tpl string, isWelcome bool, userTags, groupName string, count int) string {
	tpl = strings.TrimSpace(tpl)
	if tpl == "" {
		return ""
	}
	out := strings.ReplaceAll(tpl, "{user}", userTags)
	out = strings.ReplaceAll(out, "{group}", groupName)
	out = strings.ReplaceAll(out, "{count}", fmt.Sprintf("%d", count))
	if isWelcome {
		out = strings.ReplaceAll(out, "{event}", "welcome")
	} else {
		out = strings.ReplaceAll(out, "{event}", "goodbye")
	}
	return strings.TrimSpace(out)
}

func (b *Bot) groupName(chat types.JID) string {
	info, err := b.client.GetGroupInfo(context.Background(), chat.ToNonAD())
	if err != nil || strings.TrimSpace(info.Name) == "" {
		return "Unknown Group"
	}
	return info.Name
}

func (b *Bot) sendGroupEventCard(chat types.JID, target types.JID, delta int, welcome bool, caption string, mentions []string) error {
	groupInfo, err := b.client.GetGroupInfo(context.Background(), chat.ToNonAD())
	if err != nil {
		return err
	}
	memberCount := len(groupInfo.Participants)
	if memberCount == 0 {
		memberCount = 1
	}
	if welcome {
		memberCount += 0 // already includes joins on most events
	} else {
		memberCount += delta // leave event may already have decremented; this is best-effort
	}

	username := target.User
	avatar := ""
	if ppi, err := b.client.GetProfilePictureInfo(context.Background(), target.ToNonAD(), &whatsmeow.GetProfilePictureParams{Preview: true}); err == nil && ppi != nil {
		avatar = strings.TrimSpace(ppi.URL)
	}

	cardURL := welcomeCardAPI
	if !welcome {
		cardURL = goodbyeCardAPI
	}
	u, err := neturl.Parse(cardURL)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("username", username)
	q.Set("guildName", b.groupName(chat))
	q.Set("memberCount", fmt.Sprintf("%d", memberCount))
	q.Set("avatar", avatar)
	q.Set("background", defaultCardBG)
	q.Set("quality", "90")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("group card status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	img, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(img) == 0 {
		return errors.New("group card kosong")
	}
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mime == "" {
		mime = "image/png"
	}

	up, err := b.client.Upload(context.Background(), img, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	w, h := detectImageSize(img)
	_, err = b.client.SendMessage(context.Background(), chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mime),
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
			Width:         proto.Uint32(w),
			Height:        proto.Uint32(h),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
			},
		},
	})
	return err
}

func (b *Bot) sendCanvasImage(chat types.JID, imageURL, caption string, mentions []string) error {
	req, err := http.NewRequest(http.MethodGet, imageURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("canvas status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	img, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(img) == 0 {
		return errors.New("canvas image kosong")
	}
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mime == "" {
		mime = "image/png"
	}
	up, err := b.client.Upload(context.Background(), img, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	w, h := detectImageSize(img)
	_, err = b.client.SendMessage(context.Background(), chat, &waProto.Message{
		ImageMessage: &waProto.ImageMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mime),
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
			Width:         proto.Uint32(w),
			Height:        proto.Uint32(h),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
			},
		},
	})
	return err
}

func detectImageSize(data []byte) (uint32, uint32) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0
	}
	return uint32(cfg.Width), uint32(cfg.Height)
}

func (b *Bot) sendLevelUpCard(msg *events.Message, fromLevel, toLevel int, stats UserStats) error {
	if msg == nil {
		return nil
	}
	name := strings.TrimSpace(stats.PushName)
	if name == "" {
		name = stats.Phone
	}
	targetJID := types.NewJID(stats.Phone, types.DefaultUserServer)
	avatar := b.getProfilePictureURL(targetJID)
	if strings.TrimSpace(avatar) == "" {
		avatar = defaultAvatarURL
	}

	u, err := neturl.Parse(levelUpCardAPI)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("backgroundURL", defaultCardBG)
	q.Set("avatarURL", avatar)
	q.Set("fromLevel", fmt.Sprintf("%d", fromLevel))
	q.Set("toLevel", fmt.Sprintf("%d", toLevel))
	q.Set("name", name)
	q.Set("width", "720")
	q.Set("height", "1280")
	u.RawQuery = q.Encode()

	caption := fmt.Sprintf("🎉 Level up @%s\nLevel %d -> %d", stats.Phone, fromLevel, toLevel)
	mentions := []string{stats.Phone + "@s.whatsapp.net"}
	return b.sendCanvasImage(msg.Info.Chat, u.String(), caption, mentions)
}

func (b *Bot) handleAntiLink(msg *events.Message, text string) bool {
	if msg == nil || !msg.Info.IsGroup {
		return false
	}
	gc := b.getGroupConfig(msg.Info.Chat)
	if !gc.AntiLink || !containsURL(text) {
		return false
	}
	if !b.isBotGroupAdmin(msg.Info.Chat) {
		return false
	}
	if b.isOwner(msg) || b.isGroupAdmin(msg) {
		return false
	}

	chat := msg.Info.Chat.ToNonAD()
	sender := msg.Info.Sender.ToNonAD()
	_, err := b.client.SendMessage(context.Background(), chat, b.client.BuildRevoke(chat, sender, msg.Info.ID))
	if err != nil {
		log.Printf("gagal hapus pesan antilink: %v", err)
		return false
	}
	b.reply(msg, "Link tidak diizinkan di grup ini.")
	return true
}

func (b *Bot) isBotGroupAdmin(chat types.JID) bool {
	groupInfo, err := b.client.GetGroupInfo(context.Background(), chat.ToNonAD())
	if err != nil {
		log.Printf("gagal ambil info grup untuk cek admin bot: %v", err)
		return false
	}
	if b.client == nil || b.client.Store == nil || b.client.Store.ID == nil {
		return false
	}
	own := b.client.Store.ID.ToNonAD()
	for _, p := range groupInfo.Participants {
		jid := p.JID.ToNonAD()
		if jid.User == own.User && (p.IsAdmin || p.IsSuperAdmin) {
			return true
		}
	}
	return false
}

func (b *Bot) isGroupAdmin(msg *events.Message) bool {
	if msg == nil || !msg.Info.IsGroup {
		return false
	}
	groupInfo, err := b.client.GetGroupInfo(context.Background(), msg.Info.Chat.ToNonAD())
	if err != nil {
		log.Printf("gagal ambil info grup: %v", err)
		return false
	}

	candidates := b.messageSenders(msg)
	for _, p := range groupInfo.Participants {
		if !p.IsAdmin && !p.IsSuperAdmin {
			continue
		}
		adminIDs := []string{
			normalizePhone(p.JID.User),
			normalizePhone(p.PhoneNumber.User),
			normalizePhone(p.LID.User),
		}
		for _, c := range candidates {
			for _, adminID := range adminIDs {
				if c != "" && adminID != "" && c == adminID {
					return true
				}
			}
		}
	}
	return false
}

func (b *Bot) sendTagAll(msg *events.Message, extra string, hidden bool) error {
	groupInfo, err := b.client.GetGroupInfo(context.Background(), msg.Info.Chat.ToNonAD())
	if err != nil {
		return err
	}
	mentions := make([]string, 0, len(groupInfo.Participants))
	lines := make([]string, 0, len(groupInfo.Participants)+2)
	for _, p := range groupInfo.Participants {
		jid := p.JID.ToNonAD()
		if jid.User == "" {
			continue
		}
		mentions = append(mentions, jid.String())
		if !hidden {
			lines = append(lines, "@"+jid.User)
		}
	}

	var text string
	if hidden {
		text = extra
	} else {
		header := "Tag all"
		if extra != "" {
			header += ": " + extra
		}
		text = header + "\n" + strings.Join(lines, "\n")
	}
	return b.sendMentionMessage(msg.Info.Chat, text, mentions)
}

func (b *Bot) sendMentionMessage(chat types.JID, text string, mentions []string) error {
	_, err := b.client.SendMessage(context.Background(), chat, &waProto.Message{
		ExtendedTextMessage: &waProto.ExtendedTextMessage{
			Text: proto.String(text),
			ContextInfo: &waProto.ContextInfo{
				MentionedJID: mentions,
			},
		},
	})
	return err
}

func (b *Bot) getGroupConfig(chat types.JID) api.GroupConfig {
	return b.store.GroupConfig(groupKey(chat))
}

func (b *Bot) setGroupFeature(chat types.JID, update func(*api.GroupConfig)) error {
	return b.store.UpdateGroupConfig(groupKey(chat), update)
}

func groupKey(chat types.JID) string {
	return chat.ToNonAD().String()
}

func containsURL(s string) bool {
	return linkRegex.MatchString(s)
}

func (b *Bot) commandLimitCost(cmd string, args []string) int {
	if strings.Contains(cmd, "/") {
		parts := strings.SplitN(cmd, "/", 2)
		cmd = parts[1]
	}
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	switch cmd {
	case "toimg", "sticker", "toaudio", "tovn", "qc", "brat":
		return 1
	case "upscale", "hd":
		return 2
	case "tiktok", "tt":
		return 2
	case "instagram", "ig":
		return 2
	case "tebakbendera", "tb", "bendera", "tebakkartun", "tk", "kartun",
		"tebakkimia", "tkimia", "kimia", "tebakgambar", "tg", "gambar":
		if len(args) == 0 || strings.EqualFold(strings.TrimSpace(args[0]), "start") {
			return 1
		}
		return 0
	case "math", "maths", "mtk":
		if len(args) == 0 {
			return 1
		}
		a := strings.ToLower(strings.TrimSpace(args[0]))
		if a == "start" || isMathMode(a) {
			return 1
		}
		return 0
	default:
		return 0
	}
}

func (b *Bot) consumeLimitForCommand(msg *events.Message, cost int) error {
	if msg == nil || cost <= 0 {
		return nil
	}
	if b.isOwner(msg) {
		return nil
	}
	sender := b.commandSenderJID(msg)
	if sender.User == "" {
		return errors.New("gagal deteksi user untuk limit")
	}
	if _, err := b.store.ConsumeUserLimitByJID(sender.String(), cost); err == nil {
		return nil
	}
	current, getErr := b.store.GetUserStatsByJID(sender.String())
	if getErr != nil {
		return errors.New("limit tidak cukup")
	}
	return fmt.Errorf("limit tidak cukup (butuh %d, sisa %d). beli: .buylimit <jumlah> (1 limit = %d coins)", cost, current.Limit, limitPriceCoin)
}

func (b *Bot) extractCommand(text string) (string, string, bool) {
	prefixes := b.getPrefixes()
	sort.SliceStable(prefixes, func(i, j int) bool {
		return len(prefixes[i]) > len(prefixes[j])
	})

	for _, p := range prefixes {
		if strings.HasPrefix(text, p) {
			return p, strings.TrimSpace(strings.TrimPrefix(text, p)), true
		}
	}
	return "", "", false
}

func (b *Bot) helpText() string {
	prefixes := strings.Join(b.getPrefixes(), " ")
	return "" +
		"MEOW BOT COMMANDS\n\n" +
		"[GENERAL]\n" +
		"- ping\n" +
		"- runtime\n" +
		"- prefix\n" +
		"- profile / me [nomor]\n" +
		"- leaderboard / lb [top]\n" +
		"- daily / claim\n" +
		"- balance / wallet [nomor]\n" +
		"- work\n" +
		"- transfer <nomor> <coins>\n" +
		"- buylimit <jumlah> (1 limit = 10 coins)\n" +
		"- afk [alasan]\n" +
		"- ticket open <pesan> | ticket my | ticket info <id>\n" +
		"- tebakbendera / tb [jawaban|start|skip]\n" +
		"- tebakkartun / tk [jawaban|start|skip]\n" +
		"- tebakkimia / tkimia [jawaban|start|skip]\n" +
		"- tebakgambar / tg [jawaban|start|skip]\n" +
		"- math [mode|jawaban|skip]\n" +
		"- menu / mmenu\n\n" +
		"[OWNER]\n" +
		"- owner/addowner <nomor>\n" +
		"- owner/delowner <nomor>\n" +
		"- owner/listowner\n" +
		"- owner/setowner <nomor> (alias addowner)\n" +
		"- owner/setprefix <list>\n" +
		"- owner/setwm <pack>|<author>\n" +
		"- owner/autoread <on|off>\n" +
		"- owner/addlimit <nomor> <jumlah>\n" +
		"- owner/resetlimit <nomor>\n" +
		"- owner/dellimit <nomor>\n" +
		"- owner/setdaily <xp> <limit> <cooldown_jam>\n" +
		"- owner/ticket assign <id> <owner>\n" +
		"- $ <shell command> (owner only)\n" +
		"- x <expression> (owner only)\n\n" +
		"[GROUP]\n" +
		"- group/antilink <on|off>\n" +
		"- group/welcome <on|off>\n" +
		"- group/goodbye <on|off>\n" +
		"- group/setwelcome <template|reset>\n" +
		"- group/setgoodbye <template|reset>\n" +
		"- group/tagall [pesan]\n" +
		"- group/hidetag <pesan>\n\n" +
		"[MEDIA]\n" +
		"- toimg (reply sticker/gambar)\n" +
		"- sticker (reply gambar/video)\n" +
		"- toaudio (reply video/audio)\n" +
		"- tovn (reply video/audio)\n" +
		"- qc (reply teks / .qc teks)\n" +
		"- brat <teks>\n" +
		"- brat -animate <teks>\n" +
		"- upscale [2|4] (reply gambar/sticker)\n\n" +
		"[DOWNLOADER]\n" +
		"- tiktok <url>\n" +
		"- tt <url>\n" +
		"- instagram <url>\n" +
		"- ig <url>\n\n" +
		"prefix aktif: " + prefixes
}

func (b *Bot) isOwner(msg *events.Message) bool {
	senders := b.messageSenders(msg)
	for _, sender := range senders {
		if b.phoneInList(sender, b.superOwners) || b.phoneInList(sender, b.store.ExtraOwners()) {
			return true
		}
	}
	return false
}

func (b *Bot) isSuperOwner(msg *events.Message) bool {
	senders := b.messageSenders(msg)
	for _, sender := range senders {
		if b.phoneInList(sender, b.superOwners) {
			return true
		}
	}
	return false
}

func (b *Bot) setPrefixes(prefixes []string) error {
	return b.store.SetPrefixes(prefixes)
}

func (b *Bot) setOwner(msg *events.Message, owner string) error {
	if !b.isSuperOwner(msg) {
		return errors.New("Command ini khusus owner utama")
	}
	owner = normalizePhone(owner)
	if owner == "" {
		return errors.New("nomor owner tidak valid")
	}
	if b.phoneInList(owner, b.superOwners) {
		return nil
	}
	return b.store.AddExtraOwner(owner)
}

func (b *Bot) addOwner(msg *events.Message, owner string) error {
	return b.setOwner(msg, owner)
}

func (b *Bot) delOwner(msg *events.Message, owner string) error {
	if !b.isSuperOwner(msg) {
		return errors.New("Command ini khusus owner utama")
	}
	owner = normalizePhone(owner)
	if owner == "" {
		return errors.New("nomor owner tidak valid")
	}
	if b.phoneInList(owner, b.superOwners) {
		return errors.New("owner utama dari config.json tidak bisa dihapus lewat command")
	}
	return b.store.RemoveExtraOwner(owner)
}

func (b *Bot) listOwners() ([]string, []string) {
	super := append([]string{}, b.superOwners...)
	extra := b.store.ExtraOwners()
	return super, extra
}

func (b *Bot) setAutoRead(on bool) error {
	return b.store.SetAutoRead(on)
}

func (b *Bot) getAutoRead() bool {
	return b.store.AutoRead()
}

func (b *Bot) messageSenders(msg *events.Message) []string {
	candidates := make([]string, 0, 3)
	if msg != nil {
		if msg.Info.Sender.User != "" {
			candidates = append(candidates, msg.Info.Sender.ToNonAD().User)
		}
		if msg.Info.SenderAlt.User != "" {
			candidates = append(candidates, msg.Info.SenderAlt.ToNonAD().User)
		}
		if !msg.Info.IsGroup && msg.Info.Chat.User != "" {
			candidates = append(candidates, msg.Info.Chat.ToNonAD().User)
		}
	}

	seen := make(map[string]struct{}, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = normalizePhone(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	return out
}

func (b *Bot) trackUser(msg *events.Message) {
	if msg == nil {
		return
	}
	jid := msg.Info.Sender.ToNonAD()
	if jid.User == "" {
		jid = msg.Info.Chat.ToNonAD()
	}
	if jid.User == "" {
		return
	}

	phone := normalizePhone(jid.User)
	profile := UserProfile{
		JID:      jid.String(),
		Phone:    phone,
		PushName: strings.TrimSpace(msg.Info.PushName),
		IsOwner:  phone != "" && (b.phoneInList(phone, b.superOwners) || b.phoneInList(phone, b.store.ExtraOwners())),
	}
	if err := b.store.UpsertUser(profile); err != nil {
		log.Printf("gagal simpan user mongo: %v", err)
	}
}

func (b *Bot) handleAFKOnIncoming(msg *events.Message) {
	if msg == nil {
		return
	}
	sender := msg.Info.Sender.ToNonAD()
	if sender.User == "" {
		return
	}
	stats, err := b.store.GetUserStatsByJID(sender.String())
	if err == nil && stats.JID != "" && stats.AFK {
		_ = b.store.SetUserAFKByJID(sender.String(), "", false)
		dur := time.Since(stats.AFKSince).Round(time.Minute)
		if dur < 0 {
			dur = 0
		}
		b.reply(msg, fmt.Sprintf("✅ AFK off @%s (durasi AFK: %s)", normalizePhone(sender.User), dur))
	}

	mentioned := extractMentionedPhones(msg)
	if len(mentioned) == 0 {
		return
	}
	afkMap, err := b.store.ListAFKByPhones(mentioned)
	if err != nil || len(afkMap) == 0 {
		return
	}
	lines := []string{"🚧 User AFK:"}
	mentions := make([]string, 0, len(afkMap))
	for _, p := range mentioned {
		info, ok := afkMap[normalizePhone(p)]
		if !ok {
			continue
		}
		dur := time.Since(info.AFKSince).Round(time.Minute)
		reason := strings.TrimSpace(info.AFKReason)
		if reason == "" {
			reason = "-"
		}
		lines = append(lines, fmt.Sprintf("- @%s | %s | reason: %s", normalizePhone(p), dur, reason))
		mentions = append(mentions, normalizePhone(p)+"@s.whatsapp.net")
	}
	if len(mentions) > 0 {
		b.ReplyMention(msg, strings.Join(lines, "\n"), mentions)
	}
}

func extractMentionedPhones(msg *events.Message) []string {
	if msg == nil || msg.Message == nil {
		return nil
	}
	mentions := make([]string, 0, 4)
	add := func(values []string) {
		for _, m := range values {
			j, err := types.ParseJID(m)
			if err != nil {
				continue
			}
			if p := normalizePhone(j.User); p != "" {
				mentions = append(mentions, p)
			}
		}
	}
	switch {
	case msg.Message.GetExtendedTextMessage() != nil:
		if ci := msg.Message.GetExtendedTextMessage().GetContextInfo(); ci != nil {
			add(ci.GetMentionedJID())
		}
	case msg.Message.GetImageMessage() != nil:
		if ci := msg.Message.GetImageMessage().GetContextInfo(); ci != nil {
			add(ci.GetMentionedJID())
		}
	case msg.Message.GetVideoMessage() != nil:
		if ci := msg.Message.GetVideoMessage().GetContextInfo(); ci != nil {
			add(ci.GetMentionedJID())
		}
	case msg.Message.GetDocumentMessage() != nil:
		if ci := msg.Message.GetDocumentMessage().GetContextInfo(); ci != nil {
			add(ci.GetMentionedJID())
		}
	}
	return normalizePhoneList(mentions)
}

func (b *Bot) maybeMarkRead(msg *events.Message) {
	if !b.getAutoRead() || msg == nil || msg.Info.ID == "" {
		return
	}

	chat := msg.Info.Chat.ToNonAD()
	sender := types.EmptyJID
	if msg.Info.IsGroup || msg.Info.Chat.IsBroadcastList() {
		sender = msg.Info.Sender.ToNonAD()
	}

	readAt := msg.Info.Timestamp
	if readAt.IsZero() {
		readAt = time.Now()
	}

	ids := []types.MessageID{msg.Info.ID}
	if err := b.client.MarkRead(context.Background(), ids, readAt, chat, sender); err != nil {
		// Retry with current time to handle devices with skewed message timestamps.
		if err2 := b.client.MarkRead(context.Background(), ids, time.Now(), chat, sender); err2 != nil {
			log.Printf("gagal autoread: primary=%v retry=%v", err, err2)
		}
	}
}

func (b *Bot) runtimeText() string {
	uptime := time.Since(b.startedAt).Round(time.Second)
	return fmt.Sprintf("Runtime: %s", uptime)
}

func (b *Bot) handleProfile(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}

	var stats UserStats
	var err error
	targetPhone := ""
	targetJID := types.EmptyJID

	if len(args) > 0 {
		targetPhone, err = b.resolveTargetPhone(msg, args[0])
		if err != nil {
			return err
		}
		stats, err = b.store.GetUserStatsByPhone(targetPhone)
		if err != nil {
			return err
		}
		if stats.JID == "" {
			return errors.New("user belum terdata")
		}
		if j, perr := types.ParseJID(stats.JID); perr == nil {
			targetJID = j.ToNonAD()
		}
	} else {
		targetJID = b.commandSenderJID(msg)
		targetPhone = normalizePhone(targetJID.User)
		if targetPhone == "" {
			return errors.New("gagal deteksi pengirim command")
		}
		stats, err = b.store.GetUserStatsByPhone(targetPhone)
		if err != nil {
			return err
		}
		if stats.JID == "" {
			return errors.New("user belum terdata")
		}
	}

	level := stats.Level
	if level <= 0 {
		level = levelFromXP(stats.XP)
	}
	progress, need := expProgress(stats.XP, level)
	if progress < 0 {
		progress = 0
	}
	if need <= 0 {
		need = 100
	}
	name := strings.TrimSpace(stats.PushName)
	if name == "" {
		name = b.resolveDisplayName(targetJID)
	}
	if strings.TrimSpace(name) == "" {
		name = "user"
	}
	rankName := strings.TrimSpace(rankNameByLevel(level))
	if rankName == "" {
		rankName = "beginner"
	}
	rankID := level
	if rankID < 0 {
		rankID = 0
	}
	safeLevel := level
	if safeLevel < 1 {
		safeLevel = 1
	}

	avatar := b.resolveProfileAvatarURL(msg, targetJID, stats, targetPhone)
	if strings.TrimSpace(avatar) == "" {
		avatar = defaultAvatarURL
	}
	u, err := neturl.Parse(profileCardAPI)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("backgroundURL", defaultCardBG)
	q.Set("avatarURL", avatar)
	q.Set("rankName", rankName)
	q.Set("rankId", fmt.Sprintf("%d", rankID))
	q.Set("exp", fmt.Sprintf("%d", progress))
	q.Set("requireExp", fmt.Sprintf("%d", need))
	q.Set("level", fmt.Sprintf("%d", safeLevel))
	q.Set("name", name)
	q.Set("width", "720")
	q.Set("height", "1280")
	u.RawQuery = q.Encode()

	caption := fmt.Sprintf("👤 Profile: %s\n🏅 Rank: %s\n⭐ Level: %d\n✨ XP: %d/%d (total %d)\n🎟️ Limit: %d",
		name, rankNameByLevel(level), level, progress, need, stats.XP, stats.Limit)
	return b.sendCanvasImage(msg.Info.Chat, u.String(), caption, nil)
}

func (b *Bot) commandSenderPhone(msg *events.Message) string {
	candidates := b.messageSenders(msg)
	botPhone := ""
	if b.client != nil && b.client.Store != nil && b.client.Store.ID != nil {
		botPhone = normalizePhone(b.client.Store.ID.User)
	}
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if botPhone != "" && normalizePhone(c) == botPhone {
			continue
		}
		return normalizePhone(c)
	}
	if len(candidates) > 0 {
		return normalizePhone(candidates[0])
	}
	return ""
}

func (b *Bot) commandSenderJID(msg *events.Message) types.JID {
	if msg == nil {
		return types.EmptyJID
	}
	s := msg.Info.Sender.ToNonAD()
	if s.User != "" {
		return s
	}
	s = msg.Info.SenderAlt.ToNonAD()
	if s.User != "" {
		return s
	}
	if !msg.Info.IsGroup {
		c := msg.Info.Chat.ToNonAD()
		if c.User != "" {
			return c
		}
	}
	return types.EmptyJID
}

func (b *Bot) resolveProfileAvatarURL(msg *events.Message, targetJID types.JID, stats UserStats, targetPhone string) string {
	// 1) Best source: exact JID from message/store.
	if targetJID.User != "" {
		if url := b.getProfilePictureURL(targetJID); strings.TrimSpace(url) != "" {
			return url
		}
	}
	// 2) Stored JID fallback.
	if stats.JID != "" {
		if j, err := types.ParseJID(stats.JID); err == nil {
			if url := b.getProfilePictureURL(j.ToNonAD()); strings.TrimSpace(url) != "" {
				return url
			}
		}
	}
	// 3) Phone-based JID fallback.
	if p := normalizePhone(targetPhone); p != "" {
		if url := b.getProfilePictureURL(types.NewJID(p, types.DefaultUserServer)); strings.TrimSpace(url) != "" {
			return url
		}
	}
	// 4) If this is reply profile request, try quoted participant.
	if q := getQuotedParticipant(msg); q.User != "" {
		if url := b.getProfilePictureURL(q.ToNonAD()); strings.TrimSpace(url) != "" {
			return url
		}
	}
	return ""
}

func (b *Bot) handleLeaderboard(msg *events.Message, args []string) error {
	top := 10
	if len(args) > 0 {
		n, err := strconv.Atoi(strings.TrimSpace(args[0]))
		if err != nil || n <= 0 {
			return errors.New("format salah. Contoh: leaderboard 10")
		}
		if n > 20 {
			n = 20
		}
		top = n
	}
	list, err := b.store.TopUsersByXP(top)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return errors.New("leaderboard kosong")
	}
	lines := make([]string, 0, len(list)+2)
	mentions := make([]string, 0, len(list))
	lines = append(lines, "🏆 Leaderboard XP")
	for i, u := range list {
		phone := normalizePhone(u.Phone)
		if phone == "" {
			j, _ := types.ParseJID(u.JID)
			phone = normalizePhone(j.User)
		}
		name := strings.TrimSpace(u.PushName)
		if name == "" {
			name = phone
		}
		lines = append(lines, fmt.Sprintf("%d. %s (@%s) - Lv.%d | XP %d", i+1, name, phone, u.Level, u.XP))
		if phone != "" {
			mentions = append(mentions, phone+"@s.whatsapp.net")
		}
	}
	b.ReplyMention(msg, strings.Join(lines, "\n"), mentions)
	return nil
}

func (b *Bot) handleDaily(msg *events.Message) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	sender := msg.Info.Sender.ToNonAD()
	if sender.User == "" {
		sender = msg.Info.Chat.ToNonAD()
	}
	if sender.User == "" {
		return errors.New("gagal deteksi pengirim")
	}

	before, _ := b.store.GetUserStatsByJID(sender.String())
	dxp, dlmt, dcdh := b.store.DailyConfig()
	after, ok, wait, bonusXP, bonusLimit, streak, err := b.store.ClaimDailyByJID(sender.String(), dxp, dlmt, time.Duration(dcdh)*time.Hour)
	if err != nil {
		return err
	}
	if !ok {
		h := int(wait.Hours())
		m := int(wait.Minutes()) % 60
		return fmt.Errorf("daily sudah di-claim. coba lagi %02d:%02d jam lagi", h, m)
	}

	fromLevel := before.Level
	if fromLevel <= 0 {
		fromLevel = levelFromXP(before.XP)
	}
	toLevel := levelFromXP(after.XP)
	if toLevel > fromLevel {
		_ = b.store.SetUserLevel(sender.String(), toLevel)
		after.Level = toLevel
		_ = b.sendLevelUpCard(msg, fromLevel, toLevel, after)
	}

	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	mention := []string{}
	if phone != "" {
		mention = append(mention, phone+"@s.whatsapp.net")
	}
	text := fmt.Sprintf("🎁 Daily claim berhasil @%s\n+%d XP\n+%d Limit\n🔥 Streak: %d\n⭐ Bonus: +%d XP %+d Limit\nLevel: %d\nLimit sekarang: %d",
		phone, dxp, dlmt, streak, bonusXP, bonusLimit, after.Level, after.Limit)
	b.ReplyMention(msg, text, mention)
	return nil
}

func (b *Bot) handleBalance(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	targetPhone := ""
	if len(args) > 0 {
		targetPhone = normalizePhone(args[0])
		if targetPhone == "" {
			return errors.New("nomor target tidak valid")
		}
		stats, err := b.store.GetUserStatsByPhone(targetPhone)
		if err != nil {
			return err
		}
		if stats.JID == "" {
			return errors.New("user belum terdata")
		}
		text := fmt.Sprintf("💰 Wallet @%s\nCoins: %d\nLimit: %d\nLevel: %d\nXP: %d", targetPhone, stats.Coins, stats.Limit, stats.Level, stats.XP)
		b.ReplyMention(msg, text, []string{targetPhone + "@s.whatsapp.net"})
		return nil
	}
	sender := msg.Info.Sender.ToNonAD()
	stats, err := b.store.GetUserStatsByJID(sender.String())
	if err != nil {
		return err
	}
	phone := normalizePhone(stats.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	text := fmt.Sprintf("💰 Wallet @%s\nCoins: %d\nLimit: %d\nLevel: %d\nXP: %d", phone, stats.Coins, stats.Limit, stats.Level, stats.XP)
	b.ReplyMention(msg, text, []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleWork(msg *events.Message) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	sender := msg.Info.Sender.ToNonAD()
	stats, err := b.store.GetUserStatsByJID(sender.String())
	if err != nil {
		return err
	}
	if stats.JID == "" {
		return errors.New("user belum terdata")
	}
	if !stats.WorkAt.IsZero() && time.Since(stats.WorkAt) < workCooldown {
		wait := workCooldown - time.Since(stats.WorkAt)
		h := int(wait.Hours())
		m := int(wait.Minutes()) % 60
		return fmt.Errorf("work cooldown. coba lagi %02d:%02d", h, m)
	}
	reward := workMinCoin + int64(rand.Intn(int(workMaxCoin-workMinCoin+1)))
	after, err := b.store.AddUserCoinsByJID(sender.String(), reward)
	if err != nil {
		return err
	}
	if err := b.store.SetWorkAtByJID(sender.String(), time.Now()); err != nil {
		log.Printf("gagal set work_at: %v", err)
	}
	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	b.ReplyMention(msg, fmt.Sprintf("🛠️ @%s selesai kerja\n+%d coins\nTotal coins: %d", phone, reward, after.Coins), []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleTransfer(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	if len(args) < 2 {
		return errors.New("format salah. Contoh: transfer 628xx 100")
	}
	targetPhone, err := b.resolveTargetPhone(msg, args[0])
	if err != nil {
		return err
	}
	amount, err := strconv.ParseInt(strings.TrimSpace(args[1]), 10, 64)
	if err != nil || amount <= 0 {
		return errors.New("jumlah transfer harus angka > 0")
	}
	sender := msg.Info.Sender.ToNonAD()
	self, err := b.store.GetUserStatsByJID(sender.String())
	if err != nil {
		return err
	}
	if self.JID == "" {
		return errors.New("akun belum terdata")
	}
	if normalizePhone(self.Phone) == targetPhone {
		return errors.New("tidak bisa transfer ke diri sendiri")
	}
	if self.Coins < amount {
		return errors.New("coins tidak cukup")
	}
	allowed, wait, remain, err := b.store.CheckTransferAllowance(sender.String(), amount, transferCooldown, transferDailyMax)
	if err != nil {
		return err
	}
	if !allowed {
		if wait > 0 {
			return fmt.Errorf("transfer cooldown, coba lagi dalam %s", wait.Round(time.Second))
		}
		return fmt.Errorf("limit transfer harian terlewati. sisa kuota hari ini: %d", remain)
	}
	target, err := b.store.GetUserStatsByPhone(targetPhone)
	if err != nil {
		return err
	}
	if target.JID == "" {
		return errors.New("target belum terdata (suruh chat bot dulu)")
	}
	if _, err := b.store.AddUserCoinsByJID(sender.String(), -amount); err != nil {
		return err
	}
	toAfter, err := b.store.AddUserCoinsByJID(target.JID, amount)
	if err != nil {
		_, _ = b.store.AddUserCoinsByJID(sender.String(), amount)
		return err
	}
	after, _ := b.store.GetUserStatsByJID(sender.String())
	_ = b.store.RecordTransferOut(sender.String(), amount)
	text := fmt.Sprintf("💸 Transfer berhasil\nDari: @%s\nKe: @%s\nJumlah: %d coins\nSisa coins kamu: %d\nCoins penerima: %d",
		normalizePhone(self.Phone), targetPhone, amount, after.Coins, toAfter.Coins)
	mentions := []string{normalizePhone(self.Phone) + "@s.whatsapp.net", targetPhone + "@s.whatsapp.net"}
	b.ReplyMention(msg, text, mentions)
	return nil
}

func (b *Bot) handleBuyLimit(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	if len(args) < 1 {
		return errors.New("format salah. Contoh: buylimit 2")
	}
	qty, err := strconv.Atoi(strings.TrimSpace(args[0]))
	if err != nil || qty <= 0 {
		return errors.New("jumlah limit harus angka > 0")
	}
	cost := int64(qty) * limitPriceCoin
	sender := msg.Info.Sender.ToNonAD()
	self, err := b.store.GetUserStatsByJID(sender.String())
	if err != nil {
		return err
	}
	if self.Coins < cost {
		return fmt.Errorf("coins tidak cukup. harga %d limit = %d coins", qty, cost)
	}
	if _, err := b.store.AddUserCoinsByJID(sender.String(), -cost); err != nil {
		return err
	}
	phone := normalizePhone(self.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	after, err := b.store.AddUserLimitByPhone(phone, qty)
	if err != nil {
		_, _ = b.store.AddUserCoinsByJID(sender.String(), cost)
		return err
	}
	b.ReplyMention(msg, fmt.Sprintf("🛒 Buy limit sukses @%s\nTambah limit: %d\nBiaya: %d coins\nLimit sekarang: %d",
		phone, qty, cost, after.Limit), []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleAFK(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	sender := msg.Info.Sender.ToNonAD()
	reason := strings.TrimSpace(strings.Join(args, " "))
	if reason == "" {
		reason = "-"
	}
	if err := b.store.SetUserAFKByJID(sender.String(), reason, true); err != nil {
		return err
	}
	phone := normalizePhone(sender.User)
	b.ReplyMention(msg, fmt.Sprintf("🛌 @%s sekarang AFK\nReason: %s", phone, reason), []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleTicket(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	if len(args) == 0 {
		return errors.New("format: ticket open <pesan> | ticket my | ticket list | ticket close <id>")
	}
	action := strings.ToLower(strings.TrimSpace(args[0]))
	sender := msg.Info.Sender.ToNonAD()
	phone := normalizePhone(sender.User)
	name := strings.TrimSpace(msg.Info.PushName)
	switch action {
	case "open":
		text := strings.TrimSpace(strings.Join(args[1:], " "))
		if text == "" {
			return errors.New("isi ticket tidak boleh kosong")
		}
		t, err := b.store.CreateTicket(Ticket{
			Phone: phone,
			Name:  name,
			Chat:  msg.Info.Chat.ToNonAD().String(),
			Text:  text,
		})
		if err != nil {
			return err
		}
		b.reply(msg, fmt.Sprintf("🎫 Ticket dibuat\nID: %s\nPesan: %s", t.ID, t.Text))
		b.notifySuperOwnersNewTicket(t)
		return nil
	case "my":
		items, err := b.store.ListTicketsByPhone(phone, 10)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return errors.New("kamu belum punya ticket")
		}
		lines := []string{"🎫 Ticket kamu:"}
		for _, t := range items {
			lines = append(lines, fmt.Sprintf("- %s [%s] %s", t.ID, t.Status, t.Text))
		}
		b.reply(msg, strings.Join(lines, "\n"))
		return nil
	case "list":
		if !b.isOwner(msg) {
			return errors.New("ticket list khusus owner/admin bot")
		}
		items, err := b.store.ListTickets("open", 20)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			return errors.New("tidak ada ticket open")
		}
		lines := []string{"🎫 Ticket open:"}
		for _, t := range items {
			lines = append(lines, fmt.Sprintf("- %s | %s | @%s | %s", t.ID, t.Name, t.Phone, t.Text))
		}
		mentions := make([]string, 0, len(items))
		for _, t := range items {
			if t.Phone != "" {
				mentions = append(mentions, t.Phone+"@s.whatsapp.net")
			}
		}
		b.ReplyMention(msg, strings.Join(lines, "\n"), mentions)
		return nil
	case "info":
		if len(args) < 2 {
			return errors.New("format: ticket info <id>")
		}
		t, err := b.store.GetTicketByID(args[1])
		if err != nil {
			return err
		}
		assign := "-"
		if t.AssignedTo != "" {
			assign = "@" + t.AssignedTo
		}
		closeNote := "-"
		if strings.TrimSpace(t.CloseNote) != "" {
			closeNote = t.CloseNote
		}
		text := fmt.Sprintf("🎫 Ticket Info\nID: %s\nStatus: %s\nUser: @%s\nAssign: %s\nPesan: %s\nDibuat: %s\nClose note: %s",
			t.ID, t.Status, t.Phone, assign, t.Text, t.CreatedAt.Format("2006-01-02 15:04"), closeNote)
		mentions := []string{}
		if t.Phone != "" {
			mentions = append(mentions, t.Phone+"@s.whatsapp.net")
		}
		if t.AssignedTo != "" {
			mentions = append(mentions, t.AssignedTo+"@s.whatsapp.net")
		}
		b.ReplyMention(msg, text, mentions)
		return nil
	case "assign":
		if !b.isOwner(msg) {
			return errors.New("ticket assign khusus owner/admin bot")
		}
		if len(args) < 3 {
			return errors.New("format: ticket assign <id> <owner_number>")
		}
		id := strings.TrimSpace(args[1])
		assignTo := normalizePhone(args[2])
		if assignTo == "" {
			return errors.New("nomor owner assign tidak valid")
		}
		if !b.phoneInList(assignTo, b.superOwners) && !b.phoneInList(assignTo, b.store.ExtraOwners()) {
			return errors.New("assign hanya bisa ke owner terdaftar")
		}
		if err := b.store.AssignTicket(id, assignTo, phone); err != nil {
			return err
		}
		b.ReplyMention(msg, "ticket "+strings.ToUpper(id)+" di-assign ke @"+assignTo, []string{assignTo + "@s.whatsapp.net"})
		return nil
	case "close":
		if !b.isOwner(msg) {
			return errors.New("ticket close khusus owner/admin bot")
		}
		if len(args) < 2 {
			return errors.New("format: ticket close <id> [note]")
		}
		id := strings.TrimSpace(args[1])
		note := strings.TrimSpace(strings.Join(args[2:], " "))
		if err := b.store.CloseTicket(id, phone, note); err != nil {
			return err
		}
		b.notifySuperOwnersTicketClosed(strings.ToUpper(id), phone, note)
		if note == "" {
			b.reply(msg, "ticket ditutup: "+strings.ToUpper(id))
		} else {
			b.reply(msg, "ticket ditutup: "+strings.ToUpper(id)+"\nNote: "+note)
		}
		return nil
	default:
		return errors.New("aksi ticket tidak dikenal")
	}
}

type tebakBenderaAPIResp struct {
	Status bool `json:"status"`
	Data   struct {
		Name string `json:"name"`
		Img  string `json:"img"`
	} `json:"data"`
}

type tebakKartunAPIResp struct {
	Status bool `json:"status"`
	Data   struct {
		Name string `json:"name"`
		Img  string `json:"img"`
	} `json:"data"`
}

type tebakKimiaAPIResp struct {
	Status bool `json:"status"`
	Data   struct {
		Unsur   string `json:"unsur"`
		Lambang string `json:"lambang"`
	} `json:"data"`
}

type tebakGambarAPIResp struct {
	Status bool `json:"status"`
	Data   struct {
		Index     int    `json:"index"`
		Img       string `json:"img"`
		Jawaban   string `json:"jawaban"`
		Deskripsi string `json:"deskripsi"`
	} `json:"data"`
}

type mathGameAPIResp struct {
	Status bool `json:"status"`
	Data   struct {
		Str    string `json:"str"`
		Mode   string `json:"mode"`
		Time   int64  `json:"time"`
		Bonus  int64  `json:"bonus"`
		Result int64  `json:"result"`
	} `json:"data"`
}

func (b *Bot) handleTebakBendera(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	chatKey := msg.Info.Chat.ToNonAD().String()
	now := time.Now()

	b.flagMu.Lock()
	current, has := b.flagQuiz[chatKey]
	if has && now.Sub(current.StartedAt) > flagQuizTimeout {
		delete(b.flagQuiz, chatKey)
		has = false
	}
	b.flagMu.Unlock()

	if len(args) == 0 || strings.EqualFold(strings.TrimSpace(args[0]), "start") {
		if has {
			return errors.New("masih ada soal aktif. jawab dulu pakai .tb <jawaban> atau .tb skip")
		}
		q, err := b.fetchTebakBenderaQuestion()
		if err != nil {
			return err
		}
		answer := normalizeAnswer(q.Data.Name)
		if answer == "" {
			return errors.New("soal dari API tidak valid")
		}
		b.flagMu.Lock()
		b.flagQuiz[chatKey] = flagQuizState{
			Answer:    answer,
			StartedAt: now,
			ImageURL:  strings.TrimSpace(q.Data.Img),
		}
		b.flagMu.Unlock()

		caption := "🎮 Tebak Bendera\nJawab dengan: .tb <jawaban>\nTimeout: 2 menit\nHadiah: +150 coins"
		if err := b.sendCanvasImage(msg.Info.Chat, q.Data.Img, caption, nil); err != nil {
			b.flagMu.Lock()
			delete(b.flagQuiz, chatKey)
			b.flagMu.Unlock()
			return err
		}
		return nil
	}

	if !has {
		return errors.New("belum ada soal aktif. mulai dengan .tb")
	}

	guessRaw := strings.TrimSpace(strings.Join(args, " "))
	guess := normalizeAnswer(guessRaw)
	if guess == "" {
		return errors.New("jawaban kosong")
	}
	if guess == "skip" || guess == "nyerah" {
		b.flagMu.Lock()
		answer := b.flagQuiz[chatKey].Answer
		delete(b.flagQuiz, chatKey)
		b.flagMu.Unlock()
		return fmt.Errorf("soal di-skip. jawaban: %s", strings.ToUpper(answer))
	}

	if guess != current.Answer {
		hint := ""
		if len(current.Answer) > 0 {
			hint = string([]rune(current.Answer)[0]) + strings.Repeat("_", max(0, len([]rune(current.Answer))-1))
		}
		return fmt.Errorf("salah. hint: %s", hint)
	}

	b.flagMu.Lock()
	delete(b.flagQuiz, chatKey)
	b.flagMu.Unlock()

	sender := b.commandSenderJID(msg)
	if sender.User == "" {
		return errors.New("gagal deteksi pemenang")
	}
	after, err := b.store.AddUserCoinsByJID(sender.String(), flagQuizReward)
	if err != nil {
		return err
	}
	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	text := fmt.Sprintf("✅ Jawaban benar @%s\nHadiah: +%d coins\nTotal coins: %d", phone, flagQuizReward, after.Coins)
	b.ReplyMention(msg, text, []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) fetchTebakBenderaQuestion() (tebakBenderaAPIResp, error) {
	var out tebakBenderaAPIResp
	u := "https://api.siputzx.my.id/api/games/tebakbendera"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return out, fmt.Errorf("tebakbendera status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if !out.Status || strings.TrimSpace(out.Data.Name) == "" || strings.TrimSpace(out.Data.Img) == "" {
		return out, errors.New("response tebakbendera tidak valid")
	}
	return out, nil
}

func normalizeAnswer(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

var mathModes = map[string]struct{}{
	"noob": {}, "easy": {}, "medium": {}, "hard": {}, "extreme": {},
	"impossible": {}, "impossible2": {}, "impossible3": {}, "impossible4": {}, "impossible5": {},
}

func isMathMode(s string) bool {
	_, ok := mathModes[strings.ToLower(strings.TrimSpace(s))]
	return ok
}

func (b *Bot) handleMathGame(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	chatKey := msg.Info.Chat.ToNonAD().String()
	now := time.Now()

	b.mathMu.Lock()
	current, has := b.mathQuiz[chatKey]
	if has && now.After(current.ExpiresAt) {
		delete(b.mathQuiz, chatKey)
		has = false
	}
	b.mathMu.Unlock()

	if len(args) == 0 || strings.EqualFold(strings.TrimSpace(args[0]), "start") || (len(args) == 1 && isMathMode(args[0]) && !has) {
		if has {
			remain := time.Until(current.ExpiresAt).Round(time.Second)
			return fmt.Errorf("masih ada soal math aktif (%s). jawab pakai .math <angka> atau .math skip", remain)
		}
		mode := "easy"
		if len(args) > 0 && isMathMode(args[0]) {
			mode = strings.ToLower(strings.TrimSpace(args[0]))
		}
		q, err := b.fetchMathQuestion(mode)
		if err != nil {
			return err
		}
		timeout := q.Data.Time
		if timeout <= 0 {
			timeout = 20000
		}
		state := mathQuizState{
			Answer:    q.Data.Result,
			Mode:      q.Data.Mode,
			Bonus:     q.Data.Bonus,
			ExpiresAt: time.Now().Add(time.Duration(timeout) * time.Millisecond),
			Question:  q.Data.Str,
		}
		b.mathMu.Lock()
		b.mathQuiz[chatKey] = state
		b.mathMu.Unlock()
		return b.sendMathQuestion(msg, state, timeout)
	}

	if !has {
		return errors.New("belum ada soal math aktif. mulai dengan .math [mode]")
	}

	raw := strings.TrimSpace(strings.Join(args, " "))
	if raw == "" {
		return errors.New("jawaban kosong")
	}
	if strings.EqualFold(raw, "skip") || strings.EqualFold(raw, "nyerah") {
		b.mathMu.Lock()
		answer := b.mathQuiz[chatKey].Answer
		delete(b.mathQuiz, chatKey)
		b.mathMu.Unlock()
		return fmt.Errorf("soal math di-skip. jawaban: %d", answer)
	}
	guess, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return errors.New("jawaban harus angka. contoh: .math 42")
	}
	if guess != current.Answer {
		return errors.New("jawaban salah")
	}

	b.mathMu.Lock()
	delete(b.mathQuiz, chatKey)
	b.mathMu.Unlock()

	sender := b.commandSenderJID(msg)
	if sender.User == "" {
		return errors.New("gagal deteksi pemenang")
	}
	reward := current.Bonus
	if reward <= 0 {
		reward = 40
	}
	after, err := b.store.AddUserCoinsByJID(sender.String(), reward)
	if err != nil {
		return err
	}
	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	b.ReplyMention(msg, fmt.Sprintf("✅ Math benar @%s\nMode: %s\nHadiah: +%d coins\nTotal coins: %d",
		phone, current.Mode, reward, after.Coins), []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleTebakKartun(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	chatKey := msg.Info.Chat.ToNonAD().String()
	now := time.Now()

	b.cartoonMu.Lock()
	current, has := b.cartoonQuiz[chatKey]
	if has && now.Sub(current.StartedAt) > cartoonQuizTimeout {
		delete(b.cartoonQuiz, chatKey)
		has = false
	}
	b.cartoonMu.Unlock()

	if len(args) == 0 || strings.EqualFold(strings.TrimSpace(args[0]), "start") {
		if has {
			return errors.New("masih ada soal kartun aktif. jawab dulu pakai .tk <jawaban> atau .tk skip")
		}
		q, err := b.fetchTebakKartunQuestion()
		if err != nil {
			return err
		}
		answer := normalizeAnswer(q.Data.Name)
		if answer == "" {
			return errors.New("soal dari API tidak valid")
		}
		b.cartoonMu.Lock()
		b.cartoonQuiz[chatKey] = cartoonQuizState{
			Answer:    answer,
			StartedAt: now,
			ImageURL:  strings.TrimSpace(q.Data.Img),
		}
		b.cartoonMu.Unlock()

		caption := "🎮 Tebak Kartun\nJawab dengan: .tk <jawaban>\nTimeout: 2 menit\nHadiah: +180 coins"
		if err := b.sendCanvasImage(msg.Info.Chat, q.Data.Img, caption, nil); err != nil {
			b.cartoonMu.Lock()
			delete(b.cartoonQuiz, chatKey)
			b.cartoonMu.Unlock()
			return err
		}
		return nil
	}

	if !has {
		return errors.New("belum ada soal kartun aktif. mulai dengan .tk")
	}
	guessRaw := strings.TrimSpace(strings.Join(args, " "))
	guess := normalizeAnswer(guessRaw)
	if guess == "" {
		return errors.New("jawaban kosong")
	}
	if guess == "skip" || guess == "nyerah" {
		b.cartoonMu.Lock()
		answer := b.cartoonQuiz[chatKey].Answer
		delete(b.cartoonQuiz, chatKey)
		b.cartoonMu.Unlock()
		return fmt.Errorf("soal kartun di-skip. jawaban: %s", strings.ToUpper(answer))
	}
	if guess != current.Answer {
		hint := ""
		if len(current.Answer) > 0 {
			hint = string([]rune(current.Answer)[0]) + strings.Repeat("_", max(0, len([]rune(current.Answer))-1))
		}
		return fmt.Errorf("salah. hint: %s", hint)
	}

	b.cartoonMu.Lock()
	delete(b.cartoonQuiz, chatKey)
	b.cartoonMu.Unlock()

	sender := b.commandSenderJID(msg)
	if sender.User == "" {
		return errors.New("gagal deteksi pemenang")
	}
	after, err := b.store.AddUserCoinsByJID(sender.String(), cartoonQuizReward)
	if err != nil {
		return err
	}
	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	text := fmt.Sprintf("✅ Jawaban benar @%s\nHadiah: +%d coins\nTotal coins: %d", phone, cartoonQuizReward, after.Coins)
	b.ReplyMention(msg, text, []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleTebakKimia(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	chatKey := msg.Info.Chat.ToNonAD().String()
	now := time.Now()

	b.kimiaMu.Lock()
	current, has := b.kimiaQuiz[chatKey]
	if has && now.Sub(current.StartedAt) > kimiaQuizTimeout {
		delete(b.kimiaQuiz, chatKey)
		has = false
	}
	b.kimiaMu.Unlock()

	if len(args) == 0 || strings.EqualFold(strings.TrimSpace(args[0]), "start") {
		if has {
			return errors.New("masih ada soal kimia aktif. jawab pakai .tkimia <jawaban> atau .tkimia skip")
		}
		q, err := b.fetchTebakKimiaQuestion()
		if err != nil {
			return err
		}
		answer := normalizeAnswer(q.Data.Lambang)
		if answer == "" {
			return errors.New("soal kimia tidak valid")
		}
		state := kimiaQuizState{
			Answer:    answer,
			Question:  strings.TrimSpace(q.Data.Unsur),
			StartedAt: now,
		}
		b.kimiaMu.Lock()
		b.kimiaQuiz[chatKey] = state
		b.kimiaMu.Unlock()

		b.reply(msg, fmt.Sprintf("🧪 Tebak Kimia\nApa lambang unsur: %s ?\nJawab: .tkimia <jawaban>\nTimeout: 2 menit\nHadiah: +%d coins", state.Question, kimiaQuizReward))
		return nil
	}

	if !has {
		return errors.New("belum ada soal kimia aktif. mulai dengan .tkimia")
	}
	guess := normalizeAnswer(strings.TrimSpace(strings.Join(args, " ")))
	if guess == "" {
		return errors.New("jawaban kosong")
	}
	if guess == "skip" || guess == "nyerah" {
		b.kimiaMu.Lock()
		answer := b.kimiaQuiz[chatKey].Answer
		delete(b.kimiaQuiz, chatKey)
		b.kimiaMu.Unlock()
		return fmt.Errorf("soal kimia di-skip. jawaban: %s", strings.ToUpper(answer))
	}
	if guess != current.Answer {
		return errors.New("jawaban salah")
	}

	b.kimiaMu.Lock()
	delete(b.kimiaQuiz, chatKey)
	b.kimiaMu.Unlock()

	sender := b.commandSenderJID(msg)
	if sender.User == "" {
		return errors.New("gagal deteksi pemenang")
	}
	after, err := b.store.AddUserCoinsByJID(sender.String(), kimiaQuizReward)
	if err != nil {
		return err
	}
	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	b.ReplyMention(msg, fmt.Sprintf("✅ Tebak Kimia benar @%s\nHadiah: +%d coins\nTotal coins: %d", phone, kimiaQuizReward, after.Coins), []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) handleTebakGambar(msg *events.Message, args []string) error {
	if msg == nil {
		return errors.New("message tidak valid")
	}
	chatKey := msg.Info.Chat.ToNonAD().String()
	now := time.Now()

	b.gambarMu.Lock()
	current, has := b.gambarQuiz[chatKey]
	if has && now.Sub(current.StartedAt) > gambarQuizTimeout {
		delete(b.gambarQuiz, chatKey)
		has = false
	}
	b.gambarMu.Unlock()

	if len(args) == 0 || strings.EqualFold(strings.TrimSpace(args[0]), "start") {
		if has {
			return errors.New("masih ada soal gambar aktif. jawab pakai .tg <jawaban> atau .tg skip")
		}
		q, err := b.fetchTebakGambarQuestion()
		if err != nil {
			return err
		}
		answer := normalizeAnswer(q.Data.Jawaban)
		if answer == "" {
			return errors.New("soal gambar tidak valid")
		}
		state := gambarQuizState{
			Answer:      answer,
			ImageURL:    strings.TrimSpace(q.Data.Img),
			Description: strings.TrimSpace(q.Data.Deskripsi),
			StartedAt:   now,
		}
		b.gambarMu.Lock()
		b.gambarQuiz[chatKey] = state
		b.gambarMu.Unlock()

		caption := fmt.Sprintf("🖼️ Tebak Gambar\nJawab: .tg <jawaban>\nTimeout: 2 menit\nHadiah: +%d coins", gambarQuizReward)
		if state.Description != "" {
			caption += "\nHint: " + state.Description
		}
		if err := b.sendCanvasImage(msg.Info.Chat, state.ImageURL, caption, nil); err != nil {
			b.gambarMu.Lock()
			delete(b.gambarQuiz, chatKey)
			b.gambarMu.Unlock()
			return err
		}
		return nil
	}

	if !has {
		return errors.New("belum ada soal gambar aktif. mulai dengan .tg")
	}
	guess := normalizeAnswer(strings.TrimSpace(strings.Join(args, " ")))
	if guess == "" {
		return errors.New("jawaban kosong")
	}
	if guess == "skip" || guess == "nyerah" {
		b.gambarMu.Lock()
		answer := b.gambarQuiz[chatKey].Answer
		delete(b.gambarQuiz, chatKey)
		b.gambarMu.Unlock()
		return fmt.Errorf("soal gambar di-skip. jawaban: %s", strings.ToUpper(answer))
	}
	if guess != current.Answer {
		return errors.New("jawaban salah")
	}

	b.gambarMu.Lock()
	delete(b.gambarQuiz, chatKey)
	b.gambarMu.Unlock()

	sender := b.commandSenderJID(msg)
	if sender.User == "" {
		return errors.New("gagal deteksi pemenang")
	}
	after, err := b.store.AddUserCoinsByJID(sender.String(), gambarQuizReward)
	if err != nil {
		return err
	}
	phone := normalizePhone(after.Phone)
	if phone == "" {
		phone = normalizePhone(sender.User)
	}
	b.ReplyMention(msg, fmt.Sprintf("✅ Tebak Gambar benar @%s\nHadiah: +%d coins\nTotal coins: %d", phone, gambarQuizReward, after.Coins), []string{phone + "@s.whatsapp.net"})
	return nil
}

func (b *Bot) fetchTebakKartunQuestion() (tebakKartunAPIResp, error) {
	var out tebakKartunAPIResp
	u := "https://api.siputzx.my.id/api/games/tebakkartun"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return out, fmt.Errorf("tebakkartun status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if !out.Status || strings.TrimSpace(out.Data.Name) == "" || strings.TrimSpace(out.Data.Img) == "" {
		return out, errors.New("response tebakkartun tidak valid")
	}
	return out, nil
}

func (b *Bot) fetchTebakKimiaQuestion() (tebakKimiaAPIResp, error) {
	var out tebakKimiaAPIResp
	u := "https://api.siputzx.my.id/api/games/tebakkimia"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return out, fmt.Errorf("tebakkimia status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if !out.Status || strings.TrimSpace(out.Data.Unsur) == "" || strings.TrimSpace(out.Data.Lambang) == "" {
		return out, errors.New("response tebakkimia tidak valid")
	}
	return out, nil
}

func (b *Bot) fetchTebakGambarQuestion() (tebakGambarAPIResp, error) {
	var out tebakGambarAPIResp
	u := "https://api.siputzx.my.id/api/games/tebakgambar"
	req, err := http.NewRequest(http.MethodGet, u, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return out, fmt.Errorf("tebakgambar status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if !out.Status || strings.TrimSpace(out.Data.Img) == "" || strings.TrimSpace(out.Data.Jawaban) == "" {
		return out, errors.New("response tebakgambar tidak valid")
	}
	return out, nil
}

func (b *Bot) fetchMathQuestion(mode string) (mathGameAPIResp, error) {
	var out mathGameAPIResp
	mode = strings.ToLower(strings.TrimSpace(mode))
	if !isMathMode(mode) {
		mode = "easy"
	}
	u, err := neturl.Parse("https://api.siputzx.my.id/api/games/maths")
	if err != nil {
		return out, err
	}
	q := u.Query()
	q.Set("level", mode)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return out, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return out, fmt.Errorf("math status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return out, err
	}
	if !out.Status || strings.TrimSpace(out.Data.Str) == "" {
		return out, errors.New("response math tidak valid")
	}
	return out, nil
}

func (b *Bot) sendMathQuestion(msg *events.Message, state mathQuizState, timeoutMs int64) error {
	sec := timeoutMs / 1000
	if sec <= 0 {
		sec = 20
	}
	text := fmt.Sprintf("🧠 Math Quiz (%s)\nSoal: %s\nWaktu: %d detik\nJawab: .math <angka>\nReward: %d coins",
		state.Mode, state.Question, sec, state.Bonus)
	b.reply(msg, text)
	return nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (b *Bot) notifySuperOwnersNewTicket(t Ticket) {
	if len(b.superOwners) == 0 {
		return
	}
	text := "" +
		"🆕 Ticket Baru\n" +
		fmt.Sprintf("ID: %s\n", t.ID) +
		fmt.Sprintf("User: %s (%s)\n", t.Name, t.Phone) +
		fmt.Sprintf("Chat: %s\n", t.Chat) +
		fmt.Sprintf("Pesan: %s\n", t.Text) +
		"Gunakan command: .ticket close " + t.ID

	seen := map[string]struct{}{}
	for _, raw := range b.superOwners {
		phone := normalizePhone(raw)
		if phone == "" {
			continue
		}
		if _, ok := seen[phone]; ok {
			continue
		}
		seen[phone] = struct{}{}
		jid := types.NewJID(phone, types.DefaultUserServer)
		if _, err := b.client.SendMessage(context.Background(), jid, &waProto.Message{
			Conversation: proto.String(text),
		}); err != nil {
			log.Printf("gagal kirim notif ticket ke owner %s: %v", phone, err)
		}
	}
}

func (b *Bot) notifySuperOwnersTicketClosed(ticketID, closedBy, note string) {
	if len(b.superOwners) == 0 {
		return
	}
	text := "✅ Ticket Ditutup\n" +
		"ID: " + strings.ToUpper(strings.TrimSpace(ticketID)) + "\n" +
		"Closed by: " + closedBy + "\n"
	if strings.TrimSpace(note) != "" {
		text += "Note: " + note + "\n"
	}
	seen := map[string]struct{}{}
	for _, raw := range b.superOwners {
		phone := normalizePhone(raw)
		if phone == "" {
			continue
		}
		if _, ok := seen[phone]; ok {
			continue
		}
		seen[phone] = struct{}{}
		jid := types.NewJID(phone, types.DefaultUserServer)
		if _, err := b.client.SendMessage(context.Background(), jid, &waProto.Message{
			Conversation: proto.String(text),
		}); err != nil {
			log.Printf("gagal kirim notif close ticket ke owner %s: %v", phone, err)
		}
	}
}

func (b *Bot) resolveTargetPhone(msg *events.Message, raw string) (string, error) {
	phone := normalizePhone(strings.TrimPrefix(strings.TrimSpace(raw), "@"))
	if phone != "" {
		return phone, nil
	}
	qp := getQuotedParticipant(msg)
	if qp.User != "" {
		return normalizePhone(qp.User), nil
	}
	return "", errors.New("target tidak ditemukan. reply user atau isi nomor")
}

func (b *Bot) phoneInList(phone string, numbers []string) bool {
	phone = normalizePhone(phone)
	if phone == "" {
		return false
	}
	for _, n := range numbers {
		if normalizePhone(n) == phone {
			return true
		}
	}
	return false
}

func (b *Bot) pingText(msg *events.Message, started time.Time) string {
	hostname, _ := os.Hostname()
	uptime := time.Since(b.startedAt).Round(time.Second)
	processDelay := time.Since(started).Round(time.Millisecond)
	messageAge := time.Since(msg.Info.Timestamp).Round(time.Millisecond)
	return "" +
		"+------ PING STATUS ------+\n" +
		fmt.Sprintf("Host      : %s\n", hostname) +
		fmt.Sprintf("Go        : %s\n", runtime.Version()) +
		fmt.Sprintf("OS/Arch   : %s/%s\n", runtime.GOOS, runtime.GOARCH) +
		fmt.Sprintf("Runtime   : %s\n", uptime) +
		fmt.Sprintf("ProcDelay : %s\n", processDelay) +
		fmt.Sprintf("MsgAge    : %s\n", messageAge) +
		fmt.Sprintf("AutoRead  : %s\n", onOffText(b.getAutoRead())) +
		fmt.Sprintf("Prefix    : %s\n", strings.Join(b.getPrefixes(), " ")) +
		"+-------------------------+"
}

func onOffText(on bool) string {
	if on {
		return "ON"
	}
	return "OFF"
}

func (b *Bot) Reply(msg *events.Message, text string) { b.reply(msg, text) }
func (b *Bot) ReplyMention(msg *events.Message, text string, mentions []string) {
	if msg == nil {
		return
	}
	if err := b.sendMentionMessage(msg.Info.Chat, text, mentions); err != nil {
		log.Printf("gagal kirim mention message: %v", err)
	}
}
func (b *Bot) SendMenuButtons(msg *events.Message) error {
	prefix := "."
	if active := b.getPrefixes(); len(active) > 0 {
		prefix = active[0]
	}
	menu := "" +
		"╭──〔 MEOW BOT 〕───╮\n" +
		"│  General\n" +
		"│  • " + prefix + "ping\n" +
		"│  • " + prefix + "runtime\n" +
		"│  • " + prefix + "menu\n" +
		"│  • " + prefix + "prefix\n" +
		"│  • " + prefix + "profile\n" +
		"│  • " + prefix + "leaderboard\n" +
		"│  • " + prefix + "daily\n" +
		"│  • " + prefix + "balance\n" +
		"│  • " + prefix + "work\n" +
		"│  • " + prefix + "transfer 628xx 100\n" +
		"│  • " + prefix + "buylimit 2 (1=10 coins)\n" +
		"│  • " + prefix + "afk lagi makan\n" +
		"│  • " + prefix + "ticket open butuh bantuan\n" +
		"│  • " + prefix + "ticket my\n" +
		"│  • " + prefix + "ticket info TABC123\n" +
		"│  • " + prefix + "ticket assign TABC123 628xx\n" +
		"│  • " + prefix + "ticket close TABC123 [note]\n" +
		"│  • " + prefix + "tb / " + prefix + "tb japan\n" +
		"│  • " + prefix + "tk / " + prefix + "tk naruto\n" +
		"│  • " + prefix + "tkimia / " + prefix + "tkimia Fe\n" +
		"│  • " + prefix + "tg / " + prefix + "tg asapati\n" +
		"│  • " + prefix + "math easy / " + prefix + "math 42\n" +
		"│\n" +
		"│  Owner\n" +
		"│  • " + prefix + "addowner 628xxxx\n" +
		"│  • " + prefix + "delowner 628xxxx\n" +
		"│  • " + prefix + "listowner\n" +
		"│  • " + prefix + "setprefix .,!,#\n" +
		"│  • " + prefix + "setwm meow bot|miftah\n" +
		"│  • " + prefix + "autoread on|off\n" +
		"│  • " + prefix + "addlimit [no/reply] 5\n" +
		"│  • " + prefix + "resetlimit [no/reply]\n" +
		"│  • " + prefix + "dellimit [no/reply]\n" +
		"│  • " + prefix + "setdaily 120 3 24\n" +
		"│  • $ <shell command>\n" +
		"│  • x <expression>\n" +
		"│\n" +
		"│  Group\n" +
		"│  • " + prefix + "antilink on|off\n" +
		"│  • " + prefix + "welcome on|off\n" +
		"│  • " + prefix + "goodbye on|off\n" +
		"│  • " + prefix + "setwelcome 👋 {user} to {group}\n" +
		"│  • " + prefix + "setgoodbye 👋 {user}\n" +
		"│  • " + prefix + "tagall [pesan]\n" +
		"│  • " + prefix + "hidetag [pesan]\n" +
		"│\n" +
		"│  Media\n" +
		"│  • " + prefix + "toimg\n" +
		"│  • " + prefix + "sticker\n" +
		"│  • " + prefix + "toaudio\n" +
		"│  • " + prefix + "tovn\n" +
		"│  • " + prefix + "qc (reply teks)\n" +
		"│  • " + prefix + "brat halo\n" +
		"│  • " + prefix + "brat -animate halo\n" +
		"│  • " + prefix + "upscale 4 (reply gambar)\n" +
		"│\n" +
		"│  Downloader\n" +
		"│  • " + prefix + "tiktok <url>\n" +
		"│  • " + prefix + "tt <url>\n" +
		"│  • " + prefix + "instagram <url>\n" +
		"│  • " + prefix + "ig <url>\n" +
		"│\n" +
		"│  Prefix aktif: " + strings.Join(b.getPrefixes(), " ") + "\n" +
		"╰──────────────╯"
	_, err := b.client.SendMessage(context.Background(), msg.Info.Chat, &waProto.Message{
		Conversation: proto.String(menu),
	})
	return err
}
func (b *Bot) HelpText() string { return b.helpText() }
func (b *Bot) PingText(msg *events.Message, started time.Time) string {
	return b.pingText(msg, started)
}
func (b *Bot) RuntimeText() string { return b.runtimeText() }
func (b *Bot) HandleProfile(msg *events.Message, args []string) error {
	return b.handleProfile(msg, args)
}
func (b *Bot) HandleLeaderboard(msg *events.Message, args []string) error {
	return b.handleLeaderboard(msg, args)
}
func (b *Bot) HandleDaily(msg *events.Message) error {
	return b.handleDaily(msg)
}
func (b *Bot) HandleBalance(msg *events.Message, args []string) error {
	return b.handleBalance(msg, args)
}
func (b *Bot) HandleWork(msg *events.Message) error {
	return b.handleWork(msg)
}
func (b *Bot) HandleTransfer(msg *events.Message, args []string) error {
	return b.handleTransfer(msg, args)
}
func (b *Bot) HandleBuyLimit(msg *events.Message, args []string) error {
	return b.handleBuyLimit(msg, args)
}
func (b *Bot) HandleAFK(msg *events.Message, args []string) error {
	return b.handleAFK(msg, args)
}
func (b *Bot) HandleTicket(msg *events.Message, args []string) error {
	return b.handleTicket(msg, args)
}
func (b *Bot) HandleTebakBendera(msg *events.Message, args []string) error {
	return b.handleTebakBendera(msg, args)
}
func (b *Bot) HandleTebakKartun(msg *events.Message, args []string) error {
	return b.handleTebakKartun(msg, args)
}
func (b *Bot) HandleTebakKimia(msg *events.Message, args []string) error {
	return b.handleTebakKimia(msg, args)
}
func (b *Bot) HandleTebakGambar(msg *events.Message, args []string) error {
	return b.handleTebakGambar(msg, args)
}
func (b *Bot) HandleMathGame(msg *events.Message, args []string) error {
	return b.handleMathGame(msg, args)
}

func (b *Bot) GetPrefixes() []string { return b.getPrefixes() }
func (b *Bot) GetAutoRead() bool     { return b.getAutoRead() }
func (b *Bot) SetAutoRead(on bool) error {
	return b.setAutoRead(on)
}
func (b *Bot) GetStickerWM() (string, string) { return b.store.StickerWM() }
func (b *Bot) SetStickerWM(pack, author string) error {
	return b.store.SetStickerWM(pack, author)
}

func (b *Bot) IsOwner(msg *events.Message) bool { return b.isOwner(msg) }
func (b *Bot) IsSuperOwner(msg *events.Message) bool {
	return b.isSuperOwner(msg)
}
func (b *Bot) SetPrefixes(prefixes []string) error {
	return b.setPrefixes(prefixes)
}
func (b *Bot) SetOwner(msg *events.Message, owner string) error {
	return b.setOwner(msg, owner)
}
func (b *Bot) AddOwner(msg *events.Message, owner string) error {
	return b.addOwner(msg, owner)
}
func (b *Bot) DelOwner(msg *events.Message, owner string) error {
	return b.delOwner(msg, owner)
}
func (b *Bot) ListOwners() ([]string, []string) {
	return b.listOwners()
}
func (b *Bot) OwnerAddLimit(msg *events.Message, phone string, amount int) (int, error) {
	if !b.isSuperOwner(msg) {
		return 0, errors.New("hanya owner utama yang bisa addlimit")
	}
	stats, err := b.store.AddUserLimitByPhone(phone, amount)
	if err != nil {
		return 0, err
	}
	return stats.Limit, nil
}
func (b *Bot) OwnerResetLimit(msg *events.Message, phone string) (int, error) {
	if !b.isSuperOwner(msg) {
		return 0, errors.New("hanya owner utama yang bisa resetlimit")
	}
	stats, err := b.store.ResetUserLimitByPhone(phone)
	if err != nil {
		return 0, err
	}
	return stats.Limit, nil
}
func (b *Bot) OwnerDelLimit(msg *events.Message, phone string) (int, error) {
	if !b.isSuperOwner(msg) {
		return 0, errors.New("hanya owner utama yang bisa dellimit")
	}
	stats, err := b.store.DelUserLimitByPhone(phone)
	if err != nil {
		return 0, err
	}
	return stats.Limit, nil
}
func (b *Bot) OwnerSetDaily(msg *events.Message, xp int64, limit, cooldownHours int) error {
	if !b.isSuperOwner(msg) {
		return errors.New("hanya owner utama yang bisa setdaily")
	}
	return b.store.SetDailyConfig(xp, limit, cooldownHours)
}
func (b *Bot) GetDailyConfig() (int64, int, int) {
	return b.store.DailyConfig()
}
func (b *Bot) ResolveTargetPhone(msg *events.Message, raw string) (string, error) {
	return b.resolveTargetPhone(msg, raw)
}

func (b *Bot) ParseOnOff(s string) (bool, bool) { return parseOnOff(s) }
func (b *Bot) ParsePrefixArgs(args []string) ([]string, error) {
	return parsePrefixArgs(args)
}
func (b *Bot) NormalizePhone(s string) string { return normalizePhone(s) }

func (b *Bot) IsGroupAdmin(msg *events.Message) bool { return b.isGroupAdmin(msg) }
func (b *Bot) GetGroupConfig(chat types.JID) api.GroupConfig {
	return b.getGroupConfig(chat)
}
func (b *Bot) SetGroupFeature(chat types.JID, update func(*api.GroupConfig)) error {
	return b.setGroupFeature(chat, update)
}
func (b *Bot) SendTagAll(msg *events.Message, extra string, hidden bool) error {
	return b.sendTagAll(msg, extra, hidden)
}

func parseOnOff(s string) (bool, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "on", "1", "true", "aktif":
		return true, true
	case "off", "0", "false", "mati":
		return false, true
	default:
		return false, false
	}
}

func (b *Bot) getPrefixes() []string {
	return b.store.Prefixes()
}

func (b *Bot) reply(msg *events.Message, text string) {
	_, err := b.client.SendMessage(context.Background(), msg.Info.Chat, &waProto.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		log.Printf("gagal kirim balasan: %v", err)
	}
}

func (b *Bot) react(msg *events.Message, emoji string) error {
	if msg == nil || msg.Info.ID == "" {
		return errors.New("message invalid untuk reaction")
	}
	chat := msg.Info.Chat.ToNonAD()
	sender := msg.Info.Sender.ToNonAD()
	_, err := b.client.SendMessage(context.Background(), chat, b.client.BuildReaction(chat, sender, msg.Info.ID, emoji))
	return err
}

func parsePrefixArgs(args []string) ([]string, error) {
	if len(args) == 0 {
		return nil, errors.New("arg kosong")
	}
	items := make([]string, 0, len(args))
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			items = append(items, strings.TrimSpace(part))
		}
	}
	out := normalizePrefixes(items)
	if len(out) == 0 {
		return nil, errors.New("prefix invalid")
	}
	return out, nil
}

func normalizePrefixes(prefixes []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(prefixes))

	for _, raw := range prefixes {
		p := strings.TrimSpace(raw)
		if p == "" || len(p) > 3 {
			continue
		}

		valid := true
		for _, r := range p {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				valid = false
				break
			}
		}
		if !valid {
			continue
		}

		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}

	return out
}

func normalizePhone(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func extractText(msg *events.Message) string {
	if msg.Message == nil {
		return ""
	}

	if msg.Message.GetConversation() != "" {
		return msg.Message.GetConversation()
	}

	if msg.Message.GetExtendedTextMessage() != nil {
		return msg.Message.GetExtendedTextMessage().GetText()
	}

	if msg.Message.GetImageMessage() != nil {
		return msg.Message.GetImageMessage().GetCaption()
	}

	if msg.Message.GetVideoMessage() != nil {
		return msg.Message.GetVideoMessage().GetCaption()
	}

	if msg.Message.GetDocumentMessage() != nil {
		return msg.Message.GetDocumentMessage().GetCaption()
	}

	return ""
}
