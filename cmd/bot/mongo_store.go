package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"meow/plugins/api"
)

const (
	defaultMongoURI = "mongodb://localhost:27017"
	defaultMongoDB  = "meow_bot"
	defaultBotCfg   = "config.json"
	defaultDailyXP  = int64(120)
	defaultDailyLmt = 3
	defaultDailyCDH = 24

	mongoInitTimeout            = 12 * time.Second
	mongoServerSelectionTimeout = 8 * time.Second
	mongoConnectTimeout         = 8 * time.Second
	mongoPingTimeout            = 4 * time.Second
	mongoPingRetries            = 2
	mongoPingRetryDelay         = 800 * time.Millisecond
)

type globalSettingsDoc struct {
	ID            string    `bson:"_id"`
	Owner         string    `bson:"owner"`
	ExtraOwners   []string  `bson:"extra_owners,omitempty"`
	Prefixes      []string  `bson:"prefixes"`
	AutoRead      bool      `bson:"autoread"`
	StickerPack   string    `bson:"sticker_pack"`
	StickerAuthor string    `bson:"sticker_author"`
	DailyXP       int64     `bson:"daily_xp"`
	DailyLimit    int       `bson:"daily_limit"`
	DailyCDHours  int       `bson:"daily_cooldown_hours"`
	UpdatedAt     time.Time `bson:"updated_at"`
}

type groupSettingsDoc struct {
	ID              string    `bson:"_id"`
	AntiLink        bool      `bson:"antilink"`
	Welcome         bool      `bson:"welcome"`
	Goodbye         bool      `bson:"goodbye"`
	WelcomeTemplate string    `bson:"welcome_template,omitempty"`
	GoodbyeTemplate string    `bson:"goodbye_template,omitempty"`
	UpdatedAt       time.Time `bson:"updated_at"`
}

type userDoc struct {
	ID                  string    `bson:"_id"`
	Phone               string    `bson:"phone"`
	PushName            string    `bson:"push_name"`
	IsOwner             bool      `bson:"is_owner"`
	XP                  int64     `bson:"xp"`
	Level               int       `bson:"level"`
	Limit               int       `bson:"limit"`
	Coins               int64     `bson:"coins"`
	WorkAt              time.Time `bson:"work_at,omitempty"`
	AFK                 bool      `bson:"afk"`
	AFKReason           string    `bson:"afk_reason,omitempty"`
	AFKSince            time.Time `bson:"afk_since,omitempty"`
	DailyAt             time.Time `bson:"daily_at,omitempty"`
	DailyStreak         int       `bson:"daily_streak"`
	TransferAt          time.Time `bson:"transfer_at,omitempty"`
	TransferWindowStart time.Time `bson:"transfer_window_start,omitempty"`
	TransferOutToday    int64     `bson:"transfer_out_today"`
	UpdatedAt           time.Time `bson:"updated_at"`
	CreatedAt           time.Time `bson:"created_at"`
}

type ticketDoc struct {
	ID         string    `bson:"_id"`
	Phone      string    `bson:"phone"`
	Name       string    `bson:"name"`
	Chat       string    `bson:"chat"`
	Text       string    `bson:"text"`
	Status     string    `bson:"status"`
	CreatedAt  time.Time `bson:"created_at"`
	ClosedAt   time.Time `bson:"closed_at,omitempty"`
	ClosedBy   string    `bson:"closed_by,omitempty"`
	CloseNote  string    `bson:"close_note,omitempty"`
	AssignedTo string    `bson:"assigned_to,omitempty"`
	AssignedBy string    `bson:"assigned_by,omitempty"`
	AssignedAt time.Time `bson:"assigned_at,omitempty"`
}

type UserProfile struct {
	JID      string
	Phone    string
	PushName string
	IsOwner  bool
}

type UserStats struct {
	JID                 string
	Phone               string
	PushName            string
	IsOwner             bool
	XP                  int64
	Level               int
	Limit               int
	Coins               int64
	WorkAt              time.Time
	AFK                 bool
	AFKReason           string
	AFKSince            time.Time
	DailyAt             time.Time
	DailyStreak         int
	TransferAt          time.Time
	TransferWindowStart time.Time
	TransferOutToday    int64
	UpdatedAt           time.Time
	CreatedAt           time.Time
}

type Ticket struct {
	ID         string
	Phone      string
	Name       string
	Chat       string
	Text       string
	Status     string
	CreatedAt  time.Time
	ClosedAt   time.Time
	ClosedBy   string
	CloseNote  string
	AssignedTo string
	AssignedBy string
	AssignedAt time.Time
}

type MongoStore struct {
	client *mongo.Client
	db     *mongo.Database

	globals *mongo.Collection
	groups  *mongo.Collection
	users   *mongo.Collection
	tickets *mongo.Collection

	mu       sync.RWMutex
	owner    string
	extra    []string
	prefixes []string
	autoRead bool
	wmPack   string
	wmAuthor string
	dailyXP  int64
	dailyLmt int
	dailyCDH int
	groupCfg map[string]api.GroupConfig
}

type BotConfig struct {
	MongoURI     string   `json:"mongo_uri"`
	MongoDB      string   `json:"mongo_db"`
	Owner        string   `json:"owner"`
	OwnerNumbers []string `json:"owner_number"`
	BotNumber    string   `json:"bot_number"`
}

func NewMongoStore() (*MongoStore, error) {
	cfg, err := loadBotConfig(defaultBotCfg)
	if err != nil {
		return nil, err
	}
	uri := cfg.MongoURI
	dbName := cfg.MongoDB

	ctx, cancel := context.WithTimeout(context.Background(), mongoInitTimeout)
	defer cancel()

	clientOpts := options.Client().
		ApplyURI(uri).
		SetServerSelectionTimeout(mongoServerSelectionTimeout).
		SetConnectTimeout(mongoConnectTimeout)

	client, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return nil, fmt.Errorf("gagal connect mongo: %w", err)
	}

	var pingErr error
	for i := 0; i < mongoPingRetries; i++ {
		pingCtx, pingCancel := context.WithTimeout(context.Background(), mongoPingTimeout)
		pingErr = client.Ping(pingCtx, nil)
		pingCancel()
		if pingErr == nil {
			break
		}
		if i < mongoPingRetries-1 {
			time.Sleep(mongoPingRetryDelay)
		}
	}
	if pingErr != nil {
		_ = client.Disconnect(context.Background())
		return nil, fmt.Errorf("gagal ping mongo (%s): %w", maskMongoURI(uri), pingErr)
	}

	db := client.Database(dbName)
	store := &MongoStore{
		client:   client,
		db:       db,
		globals:  db.Collection("settings"),
		groups:   db.Collection("groups"),
		users:    db.Collection("users"),
		tickets:  db.Collection("tickets"),
		prefixes: append([]string{}, defaultPrefixes...),
		groupCfg: make(map[string]api.GroupConfig),
	}

	if err := store.load(context.Background()); err != nil {
		_ = client.Disconnect(context.Background())
		return nil, err
	}
	return store, nil
}

func loadBotConfig(path string) (BotConfig, error) {
	cfg := BotConfig{
		MongoURI:     defaultMongoURI,
		MongoDB:      defaultMongoDB,
		Owner:        "",
		OwnerNumbers: nil,
		BotNumber:    "",
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Backward compatibility: migrate old bot.json if present.
			if oldData, oldErr := os.ReadFile("bot.json"); oldErr == nil && len(oldData) > 0 {
				if err := json.Unmarshal(oldData, &cfg); err == nil {
					if cfg.MongoURI == "" {
						cfg.MongoURI = defaultMongoURI
					}
					if cfg.MongoDB == "" {
						cfg.MongoDB = defaultMongoDB
					}
					if err := saveBotConfig(path, cfg); err != nil {
						return BotConfig{}, err
					}
					return cfg, nil
				}
			}
			if err := saveBotConfig(path, cfg); err != nil {
				return BotConfig{}, err
			}
			return cfg, nil
		}
		return BotConfig{}, fmt.Errorf("gagal baca bot config: %w", err)
	}
	raw := make(map[string]any)
	if len(data) > 0 {
		if err := json.Unmarshal(data, &raw); err != nil {
			return BotConfig{}, fmt.Errorf("bot config rusak: %w", err)
		}
		if v, ok := raw["mongo_uri"].(string); ok && v != "" {
			cfg.MongoURI = v
		}
		if v, ok := raw["mongo_db"].(string); ok && v != "" {
			cfg.MongoDB = v
		}
		if v, ok := raw["owner"].(string); ok {
			cfg.Owner = normalizePhone(v)
		}
		if v, ok := raw["owner_number"].([]any); ok {
			tmp := make([]string, 0, len(v))
			for _, item := range v {
				if s, ok := item.(string); ok {
					tmp = append(tmp, s)
				}
			}
			cfg.OwnerNumbers = normalizePhoneList(tmp)
		} else if v, ok := raw["owner_number"].(string); ok && strings.TrimSpace(v) != "" {
			cfg.OwnerNumbers = normalizePhoneList([]string{v})
		}
		if v, ok := raw["bot_number"].(string); ok {
			cfg.BotNumber = normalizePhone(v)
		}
	}
	if cfg.MongoURI == "" {
		cfg.MongoURI = defaultMongoURI
	}
	if cfg.MongoDB == "" {
		cfg.MongoDB = defaultMongoDB
	}

	if len(cfg.OwnerNumbers) == 0 && cfg.Owner != "" {
		cfg.OwnerNumbers = []string{cfg.Owner}
	}

	// Preserve any existing keys in config.json and only ensure mongo keys exist/update.
	raw["mongo_uri"] = cfg.MongoURI
	raw["mongo_db"] = cfg.MongoDB
	if len(cfg.OwnerNumbers) > 0 {
		cfg.Owner = cfg.OwnerNumbers[0]
	}
	raw["owner"] = cfg.Owner // backward compatibility
	raw["owner_number"] = cfg.OwnerNumbers
	raw["bot_number"] = cfg.BotNumber
	if err := saveRawBotConfig(path, raw); err != nil {
		return BotConfig{}, err
	}
	return cfg, nil
}

func saveBotConfig(path string, cfg BotConfig) error {
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func saveRawBotConfig(path string, raw map[string]any) error {
	b, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func maskMongoURI(uri string) string {
	if uri == "" {
		return ""
	}
	if strings.HasPrefix(uri, "mongodb://") || strings.HasPrefix(uri, "mongodb+srv://") {
		if at := strings.Index(uri, "@"); at > 0 {
			if schemeEnd := strings.Index(uri, "://"); schemeEnd > 0 && at > schemeEnd+3 {
				return uri[:schemeEnd+3] + "***:***" + uri[at:]
			}
		}
	}
	return uri
}

func (s *MongoStore) load(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var global globalSettingsDoc
	err := s.globals.FindOne(ctx, bson.M{"_id": "global"}).Decode(&global)
	if err != nil {
		if err != mongo.ErrNoDocuments {
			return fmt.Errorf("gagal baca settings global: %w", err)
		}
		global = globalSettingsDoc{
			ID:            "global",
			Owner:         "",
			ExtraOwners:   nil,
			Prefixes:      append([]string{}, defaultPrefixes...),
			AutoRead:      false,
			StickerPack:   "meow bot",
			StickerAuthor: "meow",
			DailyXP:       defaultDailyXP,
			DailyLimit:    defaultDailyLmt,
			DailyCDHours:  defaultDailyCDH,
			UpdatedAt:     time.Now(),
		}
		if _, err := s.globals.InsertOne(ctx, global); err != nil {
			return fmt.Errorf("gagal init settings global: %w", err)
		}
	}

	s.mu.Lock()
	s.owner = normalizePhone(global.Owner)
	s.extra = normalizePhoneList(global.ExtraOwners)
	s.prefixes = normalizePrefixes(global.Prefixes)
	if len(s.prefixes) == 0 {
		s.prefixes = append([]string{}, defaultPrefixes...)
	}
	s.autoRead = global.AutoRead
	s.wmPack = strings.TrimSpace(global.StickerPack)
	s.wmAuthor = strings.TrimSpace(global.StickerAuthor)
	if s.wmPack == "" {
		s.wmPack = "meow bot"
	}
	if s.wmAuthor == "" {
		s.wmAuthor = "meow"
	}
	s.dailyXP = global.DailyXP
	if s.dailyXP <= 0 {
		s.dailyXP = defaultDailyXP
	}
	s.dailyLmt = global.DailyLimit
	if s.dailyLmt < 0 {
		s.dailyLmt = defaultDailyLmt
	}
	s.dailyCDH = global.DailyCDHours
	if s.dailyCDH <= 0 {
		s.dailyCDH = defaultDailyCDH
	}
	s.groupCfg = make(map[string]api.GroupConfig)
	s.mu.Unlock()

	cursor, err := s.groups.Find(ctx, bson.M{})
	if err != nil {
		return fmt.Errorf("gagal baca group settings: %w", err)
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var g groupSettingsDoc
		if err := cursor.Decode(&g); err != nil {
			continue
		}
		s.mu.Lock()
		s.groupCfg[g.ID] = api.GroupConfig{
			AntiLink:        g.AntiLink,
			Welcome:         g.Welcome,
			Goodbye:         g.Goodbye,
			WelcomeTemplate: g.WelcomeTemplate,
			GoodbyeTemplate: g.GoodbyeTemplate,
		}
		s.mu.Unlock()
	}
	return nil
}

func (s *MongoStore) Close(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	return s.client.Disconnect(ctx)
}

func (s *MongoStore) Owner() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.owner
}

func (s *MongoStore) ExtraOwners() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.extra...)
}

func (s *MongoStore) Prefixes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return append([]string{}, s.prefixes...)
}

func (s *MongoStore) AutoRead() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.autoRead
}

func (s *MongoStore) SetOwner(owner string) error {
	s.mu.Lock()
	s.owner = normalizePhone(owner)
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) AddExtraOwner(owner string) error {
	owner = normalizePhone(owner)
	if owner == "" {
		return fmt.Errorf("nomor owner tidak valid")
	}
	s.mu.Lock()
	s.extra = appendUniquePhone(s.extra, owner)
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) RemoveExtraOwner(owner string) error {
	owner = normalizePhone(owner)
	if owner == "" {
		return fmt.Errorf("nomor owner tidak valid")
	}
	s.mu.Lock()
	next := make([]string, 0, len(s.extra))
	for _, n := range s.extra {
		if n != owner {
			next = append(next, n)
		}
	}
	s.extra = next
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) SetPrefixes(prefixes []string) error {
	normalized := normalizePrefixes(prefixes)
	if len(normalized) == 0 {
		return fmt.Errorf("prefix kosong")
	}
	s.mu.Lock()
	s.prefixes = normalized
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) SetAutoRead(on bool) error {
	s.mu.Lock()
	s.autoRead = on
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) StickerWM() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.wmPack, s.wmAuthor
}

func (s *MongoStore) DailyConfig() (int64, int, int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.dailyXP, s.dailyLmt, s.dailyCDH
}

func (s *MongoStore) SetDailyConfig(xp int64, limit int, cooldownHours int) error {
	if xp <= 0 {
		return fmt.Errorf("xp harian harus > 0")
	}
	if limit < 0 {
		return fmt.Errorf("limit harian minimal 0")
	}
	if cooldownHours <= 0 {
		return fmt.Errorf("cooldown jam harus > 0")
	}
	s.mu.Lock()
	s.dailyXP = xp
	s.dailyLmt = limit
	s.dailyCDH = cooldownHours
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) SetStickerWM(pack, author string) error {
	pack = strings.TrimSpace(pack)
	author = strings.TrimSpace(author)
	if pack == "" {
		return fmt.Errorf("pack kosong")
	}
	if author == "" {
		return fmt.Errorf("author kosong")
	}
	s.mu.Lock()
	s.wmPack = pack
	s.wmAuthor = author
	err := s.persistGlobalLocked(context.Background())
	s.mu.Unlock()
	return err
}

func (s *MongoStore) GroupConfig(chat string) api.GroupConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.groupCfg[chat]
}

func (s *MongoStore) UpdateGroupConfig(chat string, update func(*api.GroupConfig)) error {
	s.mu.Lock()
	cfg := s.groupCfg[chat]
	update(&cfg)
	s.groupCfg[chat] = cfg
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.groups.ReplaceOne(ctx, bson.M{"_id": chat}, groupSettingsDoc{
		ID:              chat,
		AntiLink:        cfg.AntiLink,
		Welcome:         cfg.Welcome,
		Goodbye:         cfg.Goodbye,
		WelcomeTemplate: strings.TrimSpace(cfg.WelcomeTemplate),
		GoodbyeTemplate: strings.TrimSpace(cfg.GoodbyeTemplate),
		UpdatedAt:       time.Now(),
	}, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal simpan group settings: %w", err)
	}
	return nil
}

func (s *MongoStore) UpsertUser(profile UserProfile) error {
	if profile.JID == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.users.UpdateByID(ctx, profile.JID, bson.M{
		"$set": bson.M{
			"phone":      profile.Phone,
			"push_name":  profile.PushName,
			"is_owner":   profile.IsOwner,
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"created_at":         time.Now(),
			"xp":                 int64(0),
			"level":              1,
			"limit":              10,
			"coins":              int64(0),
			"afk":                false,
			"daily_streak":       0,
			"transfer_out_today": int64(0),
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal upsert user: %w", err)
	}
	return nil
}

func (s *MongoStore) GetUserStatsByJID(jid string) (UserStats, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var u userDoc
	if err := s.users.FindOne(ctx, bson.M{"_id": jid}).Decode(&u); err != nil {
		if err == mongo.ErrNoDocuments {
			return UserStats{}, nil
		}
		return UserStats{}, fmt.Errorf("gagal get user by jid: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) TopUsersByXP(max int) ([]UserStats, error) {
	if max <= 0 {
		max = 10
	}
	if max > 50 {
		max = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	opts := options.Find().SetSort(bson.D{{Key: "xp", Value: -1}, {Key: "updated_at", Value: 1}}).SetLimit(int64(max))
	cursor, err := s.users.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, fmt.Errorf("gagal get leaderboard: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]UserStats, 0, max)
	for cursor.Next(ctx) {
		var u userDoc
		if err := cursor.Decode(&u); err != nil {
			continue
		}
		out = append(out, userDocToStats(u))
	}
	return out, nil
}

func (s *MongoStore) GetUserStatsByPhone(phone string) (UserStats, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return UserStats{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOne().SetSort(bson.D{{Key: "updated_at", Value: -1}})
	var u userDoc
	if err := s.users.FindOne(ctx, bson.M{"phone": phone}, opts).Decode(&u); err != nil {
		if err == mongo.ErrNoDocuments {
			return UserStats{}, nil
		}
		return UserStats{}, fmt.Errorf("gagal get user by phone: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) AddUserXP(jid string, amount int64) (UserStats, error) {
	if jid == "" || amount <= 0 {
		return UserStats{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true)
	var u userDoc
	err := s.users.FindOneAndUpdate(ctx, bson.M{"_id": jid}, bson.M{
		"$inc": bson.M{
			"xp": amount,
		},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
		"$setOnInsert": bson.M{
			"created_at":         time.Now(),
			"level":              1,
			"limit":              10,
			"coins":              int64(0),
			"afk":                false,
			"daily_streak":       0,
			"transfer_out_today": int64(0),
		},
	}, opts).Decode(&u)
	if err != nil {
		return UserStats{}, fmt.Errorf("gagal add xp: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) SetUserLevel(jid string, level int) error {
	if jid == "" {
		return nil
	}
	if level < 1 {
		level = 1
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.users.UpdateByID(ctx, jid, bson.M{
		"$set": bson.M{
			"level":      level,
			"updated_at": time.Now(),
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal set user level: %w", err)
	}
	return nil
}

func (s *MongoStore) AddUserLimitByPhone(phone string, amount int) (UserStats, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return UserStats{}, fmt.Errorf("nomor tidak valid")
	}
	if amount <= 0 {
		return UserStats{}, fmt.Errorf("jumlah limit harus > 0")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOneAndUpdate().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetReturnDocument(options.After)
	var u userDoc
	err := s.users.FindOneAndUpdate(ctx, bson.M{"phone": phone}, bson.M{
		"$inc": bson.M{"limit": amount},
		"$set": bson.M{"updated_at": time.Now()},
	}, opts).Decode(&u)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return UserStats{}, fmt.Errorf("user tidak ditemukan di database")
		}
		return UserStats{}, fmt.Errorf("gagal add limit: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) ResetUserLimitByPhone(phone string) (UserStats, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return UserStats{}, fmt.Errorf("nomor tidak valid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOneAndUpdate().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetReturnDocument(options.After)
	var u userDoc
	err := s.users.FindOneAndUpdate(ctx, bson.M{"phone": phone}, bson.M{
		"$set": bson.M{"limit": 10, "updated_at": time.Now()},
	}, opts).Decode(&u)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return UserStats{}, fmt.Errorf("user tidak ditemukan di database")
		}
		return UserStats{}, fmt.Errorf("gagal reset limit: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) DelUserLimitByPhone(phone string) (UserStats, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return UserStats{}, fmt.Errorf("nomor tidak valid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOneAndUpdate().SetSort(bson.D{{Key: "updated_at", Value: -1}}).SetReturnDocument(options.After)
	var u userDoc
	err := s.users.FindOneAndUpdate(ctx, bson.M{"phone": phone}, bson.M{
		"$set": bson.M{"limit": 0, "updated_at": time.Now()},
	}, opts).Decode(&u)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return UserStats{}, fmt.Errorf("user tidak ditemukan di database")
		}
		return UserStats{}, fmt.Errorf("gagal del limit: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) ConsumeUserLimitByJID(jid string, amount int) (UserStats, error) {
	if jid == "" {
		return UserStats{}, fmt.Errorf("jid kosong")
	}
	if amount <= 0 {
		return s.GetUserStatsByJID(jid)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var u userDoc
	err := s.users.FindOneAndUpdate(ctx, bson.M{
		"_id":   jid,
		"limit": bson.M{"$gte": amount},
	}, bson.M{
		"$inc": bson.M{"limit": -amount},
		"$set": bson.M{"updated_at": time.Now()},
	}, opts).Decode(&u)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return UserStats{}, fmt.Errorf("limit tidak cukup")
		}
		return UserStats{}, fmt.Errorf("gagal consume limit: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) ClaimDailyByJID(jid string, addXP int64, addLimit int, cooldown time.Duration) (UserStats, bool, time.Duration, int64, int, int, error) {
	if jid == "" {
		return UserStats{}, false, 0, 0, 0, 0, fmt.Errorf("jid kosong")
	}
	if addXP < 0 {
		addXP = 0
	}
	if addLimit < 0 {
		addLimit = 0
	}
	now := time.Now()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	var existing userDoc
	err := s.users.FindOne(ctx, bson.M{"_id": jid}).Decode(&existing)
	if err != nil && err != mongo.ErrNoDocuments {
		return UserStats{}, false, 0, 0, 0, 0, fmt.Errorf("gagal cek daily: %w", err)
	}

	if cooldown <= 0 {
		cooldown = 24 * time.Hour
	}
	if err == nil && !existing.DailyAt.IsZero() {
		next := existing.DailyAt.Add(cooldown)
		if now.Before(next) {
			return userDocToStats(existing), false, next.Sub(now), 0, 0, existing.DailyStreak, nil
		}
	}

	streak := 1
	if err == nil {
		last := existing.DailyAt
		if !last.IsZero() {
			y1, m1, d1 := last.Date()
			y2, m2, d2 := now.Date()
			days := int(time.Date(y2, m2, d2, 0, 0, 0, 0, now.Location()).Sub(time.Date(y1, m1, d1, 0, 0, 0, 0, now.Location())).Hours() / 24)
			switch {
			case days == 1:
				streak = existing.DailyStreak + 1
			case days <= 0:
				streak = existing.DailyStreak
				if streak < 1 {
					streak = 1
				}
			default:
				streak = 1
			}
		}
	}
	if streak < 1 {
		streak = 1
	}

	bonusXP := int64((streak - 1) * 20)
	if bonusXP > 200 {
		bonusXP = 200
	}
	bonusLimit := 0
	if streak > 0 && streak%7 == 0 {
		bonusLimit = 1
	}
	totalXP := addXP + bonusXP
	totalLimit := addLimit + bonusLimit

	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true)
	var after userDoc
	err = s.users.FindOneAndUpdate(ctx, bson.M{"_id": jid}, bson.M{
		"$inc": bson.M{
			"xp":    totalXP,
			"limit": totalLimit,
		},
		"$set": bson.M{
			"daily_at":     now,
			"daily_streak": streak,
			"updated_at":   now,
		},
		"$setOnInsert": bson.M{
			"created_at":         now,
			"level":              1,
			"transfer_out_today": int64(0),
		},
	}, opts).Decode(&after)
	if err != nil {
		return UserStats{}, false, 0, 0, 0, 0, fmt.Errorf("gagal claim daily: %w", err)
	}
	return userDocToStats(after), true, 0, bonusXP, bonusLimit, streak, nil
}

func userDocToStats(u userDoc) UserStats {
	level := u.Level
	if level <= 0 {
		level = 1
	}
	return UserStats{
		JID:                 u.ID,
		Phone:               u.Phone,
		PushName:            u.PushName,
		IsOwner:             u.IsOwner,
		XP:                  u.XP,
		Level:               level,
		Limit:               u.Limit,
		Coins:               u.Coins,
		WorkAt:              u.WorkAt,
		AFK:                 u.AFK,
		AFKReason:           u.AFKReason,
		AFKSince:            u.AFKSince,
		DailyAt:             u.DailyAt,
		DailyStreak:         u.DailyStreak,
		TransferAt:          u.TransferAt,
		TransferWindowStart: u.TransferWindowStart,
		TransferOutToday:    u.TransferOutToday,
		UpdatedAt:           u.UpdatedAt,
		CreatedAt:           u.CreatedAt,
	}
}

func (s *MongoStore) AddUserCoinsByJID(jid string, amount int64) (UserStats, error) {
	if jid == "" || amount == 0 {
		return UserStats{}, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After).SetUpsert(true)
	var u userDoc
	err := s.users.FindOneAndUpdate(ctx, bson.M{"_id": jid}, bson.M{
		"$inc": bson.M{"coins": amount},
		"$set": bson.M{"updated_at": time.Now()},
		"$setOnInsert": bson.M{
			"created_at":         time.Now(),
			"xp":                 int64(0),
			"level":              1,
			"limit":              10,
			"afk":                false,
			"daily_streak":       0,
			"transfer_out_today": int64(0),
		},
	}, opts).Decode(&u)
	if err != nil {
		return UserStats{}, fmt.Errorf("gagal update coins: %w", err)
	}
	return userDocToStats(u), nil
}

func (s *MongoStore) SetUserAFKByJID(jid, reason string, on bool) error {
	if jid == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	set := bson.M{
		"afk":        on,
		"updated_at": time.Now(),
	}
	if on {
		set["afk_reason"] = strings.TrimSpace(reason)
		set["afk_since"] = time.Now()
	} else {
		set["afk_reason"] = ""
		set["afk_since"] = time.Time{}
	}
	_, err := s.users.UpdateByID(ctx, jid, bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"created_at":         time.Now(),
			"xp":                 int64(0),
			"level":              1,
			"limit":              10,
			"coins":              int64(0),
			"daily_streak":       0,
			"transfer_out_today": int64(0),
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal set afk: %w", err)
	}
	return nil
}

func (s *MongoStore) ListAFKByPhones(phones []string) (map[string]UserStats, error) {
	out := make(map[string]UserStats)
	if len(phones) == 0 {
		return out, nil
	}
	normalized := normalizePhoneList(phones)
	if len(normalized) == 0 {
		return out, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	cursor, err := s.users.Find(ctx, bson.M{
		"phone": bson.M{"$in": normalized},
		"afk":   true,
	})
	if err != nil {
		return nil, fmt.Errorf("gagal list afk: %w", err)
	}
	defer cursor.Close(ctx)
	for cursor.Next(ctx) {
		var u userDoc
		if err := cursor.Decode(&u); err != nil {
			continue
		}
		out[normalizePhone(u.Phone)] = userDocToStats(u)
	}
	return out, nil
}

func (s *MongoStore) SetWorkAtByJID(jid string, at time.Time) error {
	if jid == "" {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.users.UpdateByID(ctx, jid, bson.M{
		"$set": bson.M{
			"work_at":    at,
			"updated_at": time.Now(),
		},
	}, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal set work time: %w", err)
	}
	return nil
}

func (s *MongoStore) CheckTransferAllowance(jid string, amount int64, cooldown time.Duration, dailyMax int64) (bool, time.Duration, int64, error) {
	stats, err := s.GetUserStatsByJID(jid)
	if err != nil {
		return false, 0, 0, err
	}
	now := time.Now()
	if cooldown > 0 && !stats.TransferAt.IsZero() && now.Sub(stats.TransferAt) < cooldown {
		return false, cooldown - now.Sub(stats.TransferAt), 0, nil
	}
	spent := stats.TransferOutToday
	window := stats.TransferWindowStart
	if window.IsZero() || now.Sub(window) >= 24*time.Hour {
		spent = 0
	}
	if dailyMax > 0 && spent+amount > dailyMax {
		return false, 0, dailyMax - spent, nil
	}
	return true, 0, dailyMax - spent, nil
}

func (s *MongoStore) RecordTransferOut(jid string, amount int64) error {
	if jid == "" || amount <= 0 {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	stats, err := s.GetUserStatsByJID(jid)
	if err != nil {
		return err
	}
	now := time.Now()
	update := bson.M{
		"$set": bson.M{
			"transfer_at": now,
			"updated_at":  now,
		},
		"$inc": bson.M{
			"transfer_out_today": amount,
		},
	}
	if stats.TransferWindowStart.IsZero() || now.Sub(stats.TransferWindowStart) >= 24*time.Hour {
		update = bson.M{
			"$set": bson.M{
				"transfer_at":           now,
				"transfer_window_start": now,
				"transfer_out_today":    amount,
				"updated_at":            now,
			},
		}
	}
	_, err = s.users.UpdateByID(ctx, jid, update, options.Update().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal record transfer out: %w", err)
	}
	return nil
}

func (s *MongoStore) CreateTicket(t Ticket) (Ticket, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	now := time.Now()
	t.ID = strings.ToUpper(fmt.Sprintf("T%s", strconv.FormatInt(now.UnixNano(), 36)))
	t.Status = "open"
	t.CreatedAt = now
	doc := ticketDoc{
		ID:        t.ID,
		Phone:     normalizePhone(t.Phone),
		Name:      strings.TrimSpace(t.Name),
		Chat:      t.Chat,
		Text:      strings.TrimSpace(t.Text),
		Status:    t.Status,
		CreatedAt: t.CreatedAt,
	}
	if _, err := s.tickets.InsertOne(ctx, doc); err != nil {
		return Ticket{}, fmt.Errorf("gagal create ticket: %w", err)
	}
	return t, nil
}

func (s *MongoStore) ListTickets(status string, max int64) ([]Ticket, error) {
	if max <= 0 {
		max = 10
	}
	filter := bson.M{}
	if status != "" {
		filter["status"] = status
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(max)
	cursor, err := s.tickets.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("gagal list ticket: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]Ticket, 0, max)
	for cursor.Next(ctx) {
		var d ticketDoc
		if err := cursor.Decode(&d); err != nil {
			continue
		}
		out = append(out, Ticket{
			ID:         d.ID,
			Phone:      d.Phone,
			Name:       d.Name,
			Chat:       d.Chat,
			Text:       d.Text,
			Status:     d.Status,
			CreatedAt:  d.CreatedAt,
			ClosedAt:   d.ClosedAt,
			ClosedBy:   d.ClosedBy,
			CloseNote:  d.CloseNote,
			AssignedTo: d.AssignedTo,
			AssignedBy: d.AssignedBy,
			AssignedAt: d.AssignedAt,
		})
	}
	return out, nil
}

func (s *MongoStore) ListTicketsByPhone(phone string, max int64) ([]Ticket, error) {
	phone = normalizePhone(phone)
	if phone == "" {
		return nil, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}).SetLimit(max)
	cursor, err := s.tickets.Find(ctx, bson.M{"phone": phone}, opts)
	if err != nil {
		return nil, fmt.Errorf("gagal list ticket by phone: %w", err)
	}
	defer cursor.Close(ctx)
	out := make([]Ticket, 0, max)
	for cursor.Next(ctx) {
		var d ticketDoc
		if err := cursor.Decode(&d); err != nil {
			continue
		}
		out = append(out, Ticket{
			ID:         d.ID,
			Phone:      d.Phone,
			Name:       d.Name,
			Chat:       d.Chat,
			Text:       d.Text,
			Status:     d.Status,
			CreatedAt:  d.CreatedAt,
			ClosedAt:   d.ClosedAt,
			ClosedBy:   d.ClosedBy,
			CloseNote:  d.CloseNote,
			AssignedTo: d.AssignedTo,
			AssignedBy: d.AssignedBy,
			AssignedAt: d.AssignedAt,
		})
	}
	return out, nil
}

func (s *MongoStore) GetTicketByID(id string) (Ticket, error) {
	id = strings.TrimSpace(strings.ToUpper(id))
	if id == "" {
		return Ticket{}, fmt.Errorf("id ticket kosong")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var d ticketDoc
	if err := s.tickets.FindOne(ctx, bson.M{"_id": id}).Decode(&d); err != nil {
		if err == mongo.ErrNoDocuments {
			return Ticket{}, fmt.Errorf("ticket tidak ditemukan")
		}
		return Ticket{}, fmt.Errorf("gagal get ticket: %w", err)
	}
	return Ticket{
		ID:         d.ID,
		Phone:      d.Phone,
		Name:       d.Name,
		Chat:       d.Chat,
		Text:       d.Text,
		Status:     d.Status,
		CreatedAt:  d.CreatedAt,
		ClosedAt:   d.ClosedAt,
		ClosedBy:   d.ClosedBy,
		CloseNote:  d.CloseNote,
		AssignedTo: d.AssignedTo,
		AssignedBy: d.AssignedBy,
		AssignedAt: d.AssignedAt,
	}, nil
}

func (s *MongoStore) AssignTicket(id, assignedTo, assignedBy string) error {
	id = strings.TrimSpace(strings.ToUpper(id))
	if id == "" {
		return fmt.Errorf("id ticket kosong")
	}
	assignedTo = normalizePhone(assignedTo)
	if assignedTo == "" {
		return fmt.Errorf("nomor owner assign tidak valid")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.tickets.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"assigned_to": assignedTo,
			"assigned_by": normalizePhone(assignedBy),
			"assigned_at": time.Now(),
		},
	})
	if err != nil {
		return fmt.Errorf("gagal assign ticket: %w", err)
	}
	return nil
}

func (s *MongoStore) CloseTicket(id, closedBy, note string) error {
	id = strings.TrimSpace(strings.ToUpper(id))
	if id == "" {
		return fmt.Errorf("id ticket kosong")
	}
	note = strings.TrimSpace(note)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.tickets.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"status":     "closed",
			"closed_at":  time.Now(),
			"closed_by":  normalizePhone(closedBy),
			"close_note": note,
		},
	})
	if err != nil {
		return fmt.Errorf("gagal close ticket: %w", err)
	}
	return nil
}

func (s *MongoStore) persistGlobalLocked(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	_, err := s.globals.ReplaceOne(ctx, bson.M{"_id": "global"}, globalSettingsDoc{
		ID:            "global",
		Owner:         s.owner,
		ExtraOwners:   append([]string{}, s.extra...),
		Prefixes:      append([]string{}, s.prefixes...),
		AutoRead:      s.autoRead,
		StickerPack:   s.wmPack,
		StickerAuthor: s.wmAuthor,
		DailyXP:       s.dailyXP,
		DailyLimit:    s.dailyLmt,
		DailyCDHours:  s.dailyCDH,
		UpdatedAt:     time.Now(),
	}, options.Replace().SetUpsert(true))
	if err != nil {
		return fmt.Errorf("gagal simpan settings global: %w", err)
	}
	return nil
}

func normalizePhoneList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := make(map[string]struct{}, len(in))
	for _, s := range in {
		n := normalizePhone(s)
		if n == "" {
			continue
		}
		if _, ok := seen[n]; ok {
			continue
		}
		seen[n] = struct{}{}
		out = append(out, n)
	}
	return out
}

func appendUniquePhone(list []string, owner string) []string {
	for _, n := range list {
		if n == owner {
			return list
		}
	}
	return append(list, owner)
}
