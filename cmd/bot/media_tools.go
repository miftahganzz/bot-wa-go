package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"go.mau.fi/whatsmeow"
	waProto "go.mau.fi/whatsmeow/binary/proto"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"google.golang.org/protobuf/proto"
)

const tikwmAPIBase = "https://tikwm.com/api/"
const instagramFastDLAPIBase = "https://api.siputzx.my.id/api/d/fastdl"
const qcAPIBase = "https://brat.siputzx.my.id/quoted"
const bratImageAPIBase = "https://brat.siputzx.my.id/image"
const bratGIFAPIBase = "https://brat.siputzx.my.id/gif"
const upscaleAPIBase = "https://api.siputzx.my.id/api/tools/upscale"
const litterboxUploadAPI = "https://litterbox.catbox.moe/resources/internals/api.php"

type tikwmResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		ID        string   `json:"id"`
		Region    string   `json:"region"`
		Title     string   `json:"title"`
		Duration  int      `json:"duration"`
		Play      string   `json:"play"`
		WMPlay    string   `json:"wmplay"`
		Images    []string `json:"images"`
		Music     string   `json:"music"`
		MusicInfo struct {
			Play   string `json:"play"`
			Title  string `json:"title"`
			Author string `json:"author"`
		} `json:"music_info"`
		PlayCount    int64 `json:"play_count"`
		DiggCount    int64 `json:"digg_count"`
		CommentCount int64 `json:"comment_count"`
		ShareCount   int64 `json:"share_count"`
		CreateTime   int64 `json:"create_time"`
		Author       struct {
			UniqueID string `json:"unique_id"`
			Nickname string `json:"nickname"`
		} `json:"author"`
	} `json:"data"`
}

type instagramFastDLResponse struct {
	Status bool            `json:"status"`
	Data   json.RawMessage `json:"data"`
}

type instagramFastDLMedia struct {
	URL  string `json:"url"`
	Name string `json:"name"`
	Type string `json:"type"`
	Ext  string `json:"ext"`
}

type instagramFastDLMeta struct {
	Title    string `json:"title"`
	Source   string `json:"source"`
	Username string `json:"username"`
	TakenAt  int64  `json:"taken_at"`
}

type instagramFastDLSingleData struct {
	URL          []instagramFastDLMedia `json:"url"`
	Meta         instagramFastDLMeta    `json:"meta"`
	Thumb        string                 `json:"thumb"`
	ResponseType string                 `json:"response_type"`
	ContentType  string                 `json:"content_type"`
	Success      *bool                  `json:"success"`
	Message      string                 `json:"message"`
}

type instagramFastDLStoryData struct {
	URL   []instagramFastDLMedia `json:"url"`
	Meta  instagramFastDLMeta    `json:"meta"`
	Thumb string                 `json:"thumb"`
}

func (b *Bot) HandleToImg(msg *events.Message) (retErr error) {
	if err := requireFFmpeg(); err != nil {
		return err
	}
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	quoted := getQuotedMessage(msg)
	if quoted == nil {
		return errors.New("quoted message tidak ditemukan")
	}

	data, inputExt, _, err := downloadQuotedMedia(b, quoted, []string{"sticker", "image"})
	if err != nil {
		return err
	}

	in, out, cleanup, err := makeTempFiles(inputExt, ".jpg")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.WriteFile(in, data, 0o600); err != nil {
		return err
	}
	if err := runFFmpeg("-y", "-i", in, "-frames:v", "1", out); err != nil {
		return err
	}

	img, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return b.sendImageFromBytes(msg, img, "image/jpeg", "Done toimg")
}

func (b *Bot) HandleSticker(msg *events.Message) (retErr error) {
	if err := requireFFmpeg(); err != nil {
		return err
	}
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	source := getStickerSourceMessage(msg)
	if source == nil {
		return errors.New("reply media atau kirim gambar/video dengan caption command")
	}

	data, inputExt, mediaKind, err := downloadQuotedMedia(b, source, []string{"image", "video", "sticker"})
	if err != nil {
		return err
	}

	in, out, cleanup, err := makeTempFiles(inputExt, ".webp")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.WriteFile(in, data, 0o600); err != nil {
		return err
	}

	vf := "scale=512:512:force_original_aspect_ratio=decrease,fps=15,pad=512:512:-1:-1:color=white@0.0"
	isVideoInput := strings.EqualFold(filepath.Ext(in), ".mp4") || strings.EqualFold(filepath.Ext(in), ".mov") || strings.EqualFold(filepath.Ext(in), ".webm")
	if err := convertToWebP(in, out, vf, isVideoInput); err != nil {
		return err
	}

	stickerBytes, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	animated := mediaKind == "video"
	return b.sendStickerFromBytes(msg, stickerBytes, animated)
}

func (b *Bot) HandleToAudio(msg *events.Message) (retErr error) {
	if err := requireFFmpeg(); err != nil {
		return err
	}
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	source := getToAudioSourceMessage(msg)
	if source == nil {
		return errors.New("reply media atau kirim video/audio dengan caption command")
	}

	data, inputExt, _, err := downloadQuotedMedia(b, source, []string{"video", "audio"})
	if err != nil {
		return err
	}

	in, out, cleanup, err := makeTempFiles(inputExt, ".m4a")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.WriteFile(in, data, 0o600); err != nil {
		return err
	}
	if err := convertToAudio(in, out); err != nil {
		return err
	}

	audioBytes, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return b.sendAudioFromBytes(msg, audioBytes, "audio/mp4", false)
}

func (b *Bot) HandleToVN(msg *events.Message) (retErr error) {
	if err := requireFFmpeg(); err != nil {
		return err
	}
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	source := getToAudioSourceMessage(msg)
	if source == nil {
		return errors.New("reply media atau kirim video/audio dengan caption command")
	}

	data, inputExt, _, err := downloadQuotedMedia(b, source, []string{"video", "audio"})
	if err != nil {
		return err
	}

	in, out, cleanup, err := makeTempFiles(inputExt, ".ogg")
	if err != nil {
		return err
	}
	defer cleanup()

	if err := os.WriteFile(in, data, 0o600); err != nil {
		return err
	}
	if err := convertToVoiceNote(in, out); err != nil {
		return err
	}

	audioBytes, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return b.sendAudioFromBytes(msg, audioBytes, "audio/ogg; codecs=opus", true)
}

func (b *Bot) HandleQC(msg *events.Message, args []string) (retErr error) {
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	quoted := getQuotedMessage(msg)
	mainText := strings.TrimSpace(strings.Join(args, " "))

	senderJID := msg.Info.Sender.ToNonAD()
	senderName := strings.TrimSpace(msg.Info.PushName)
	if senderName == "" {
		senderName = b.resolveDisplayName(senderJID)
	}
	if senderName == "" {
		senderName = "@" + senderJID.User
	}
	avatarURL := b.getProfilePictureURL(senderJID)

	replyName := ""
	replyText := ""
	if quoted != nil {
		replyText = strings.TrimSpace(extractTextFromProtoMessage(quoted))
		if replyText == "" && hasMediaInProto(quoted) {
			replyText = "[media]"
		}
		if pjid := getQuotedParticipant(msg); !pjid.IsEmpty() {
			replyName = b.resolveDisplayName(pjid)
			if replyName == "" {
				replyName = "@" + pjid.User
			}
		}
		if replyName == "" {
			replyName = "quoted"
		}
		// In reply mode, always create "reply quote" style (our bubble replying to quoted bubble).
		if mainText == "" {
			mainText = "..."
		}
	}

	mediaURL, mediaType := "", ""
	var err error
	if quoted == nil {
		// Media is only accepted from current message (image + caption .qc ...), not from replied message.
		mediaURL, mediaType, err = extractCurrentImageMediaInput(b, msg)
		if err != nil {
			return err
		}
		if mediaURL != "" && mainText == "" {
			return errors.New("untuk qc media, wajib image + caption .qc <teks>")
		}
		if mainText == "" && mediaURL == "" {
			return errors.New("ketik .qc <teks> atau kirim image + caption .qc <teks>")
		}
	} else if mainText == "" && replyText == "" {
		return errors.New("reply tidak punya teks")
	}

	pngBytes, err := requestQuoteCard(senderName, avatarURL, mainText, mediaURL, mediaType, replyName, replyText)
	if err != nil {
		return err
	}

	in, out, cleanup, err := makeTempFiles(".png", ".webp")
	if err != nil {
		return err
	}
	defer cleanup()
	if err := os.WriteFile(in, pngBytes, 0o600); err != nil {
		return err
	}

	// Keep QC canvas as-is from API, don't resize down and don't add any borders.
	vf := "scale=iw:ih"
	if err := convertToWebP(in, out, vf, false); err != nil {
		return err
	}
	stickerBytes, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return b.sendStickerFromBytes(msg, stickerBytes, false)
}

func (b *Bot) HandleBrat(msg *events.Message, text string) (retErr error) {
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("text kosong")
	}

	u, err := neturl.Parse(bratImageAPIBase)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("text", text)
	q.Set("background", "#ffffff")
	q.Set("color", "#000000")
	q.Set("emojiStyle", "apple")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "meow-bot/1.0")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("brat api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("brat api body kosong")
	}

	in, out, cleanup, err := makeTempFiles(".png", ".webp")
	if err != nil {
		return err
	}
	defer cleanup()
	if err := os.WriteFile(in, data, 0o600); err != nil {
		return err
	}
	vf := "scale=512:512:force_original_aspect_ratio=decrease"
	if err := convertToWebP(in, out, vf, false); err != nil {
		return err
	}
	stickerBytes, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return b.sendStickerFromBytes(msg, stickerBytes, false)
}

func (b *Bot) HandleBratAnimate(msg *events.Message, text string) (retErr error) {
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("text kosong")
	}

	u, err := neturl.Parse(bratGIFAPIBase)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("text", text)
	q.Set("background", "#ffffff")
	q.Set("color", "#000000")
	q.Set("emojiStyle", "apple")
	q.Set("delay", "500")
	q.Set("endPlay", "1000")
	q.Set("width", "352")
	q.Set("height", "352")
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "image/gif,image/*")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("brat gif api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	gifBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(gifBytes) == 0 {
		return errors.New("brat gif body kosong")
	}

	in, out, cleanup, err := makeTempFiles(".gif", ".webp")
	if err != nil {
		return err
	}
	defer cleanup()
	if err := os.WriteFile(in, gifBytes, 0o600); err != nil {
		return err
	}
	vf := "fps=15,scale=352:352:force_original_aspect_ratio=decrease"
	if err := convertToWebP(in, out, vf, true); err != nil {
		return err
	}
	stickerBytes, err := os.ReadFile(out)
	if err != nil {
		return err
	}
	return b.sendStickerFromBytes(msg, stickerBytes, true)
}

func (b *Bot) HandleUpscale(msg *events.Message, args []string) (retErr error) {
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	scale := "4"
	if len(args) > 0 {
		s := strings.TrimSpace(args[0])
		if s != "" {
			scale = s
		}
	}
	if scale != "2" && scale != "4" {
		return errors.New("scale hanya support 2 atau 4")
	}

	source := getUpscaleSourceMessage(msg)
	if source == nil {
		return errors.New("reply gambar/sticker atau kirim gambar dengan caption command")
	}
	data, ext, _, err := downloadQuotedMedia(b, source, []string{"image", "sticker"})
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return errors.New("gambar kosong")
	}
	if ext == "" || ext == ".bin" {
		ext = ".jpg"
	}
	url, err := uploadToLitterbox(data, "upscale"+ext)
	if err != nil {
		return err
	}

	u, err := neturl.Parse(upscaleAPIBase)
	if err != nil {
		return err
	}
	q := u.Query()
	q.Set("url", url)
	q.Set("scale", scale)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "image/*")
	req.Header.Set("User-Agent", "meow-bot/1.0")
	resp, err := (&http.Client{Timeout: 90 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("upscale status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	img, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if len(img) == 0 {
		return errors.New("upscale result kosong")
	}
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	if mime == "" {
		mime = "image/png"
	}
	return b.sendImageFromBytes(msg, img, mime, "✅ Upscale x"+scale)
}

func uploadToLitterbox(data []byte, filename string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	_ = writer.WriteField("reqtype", "fileupload")
	_ = writer.WriteField("time", "1h")
	_ = writer.WriteField("fileNameLength", "6")
	part, err := writer.CreateFormFile("fileToUpload", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, litterboxUploadAPI, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json,text/plain,*/*")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := (&http.Client{Timeout: 60 * time.Second}).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	text := strings.TrimSpace(string(respBody))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("litterbox status %d: %s", resp.StatusCode, text)
	}
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return text, nil
	}
	// fallback: find first URL in body
	for _, field := range strings.Fields(text) {
		if strings.HasPrefix(field, "http://") || strings.HasPrefix(field, "https://") {
			return field, nil
		}
	}
	return "", fmt.Errorf("upload litterbox gagal: %s", text)
}

func (b *Bot) getProfilePictureURL(jid types.JID) string {
	if jid.User == "" {
		return ""
	}
	try := []whatsmeow.GetProfilePictureParams{
		{Preview: false},
		{Preview: true},
	}
	for _, p := range try {
		info, err := b.client.GetProfilePictureInfo(context.Background(), jid.ToNonAD(), &p)
		if err != nil || info == nil {
			continue
		}
		url := strings.TrimSpace(info.URL)
		if url != "" {
			return url
		}
	}
	return ""
}

func extractCurrentImageMediaInput(b *Bot, msg *events.Message) (string, string, error) {
	if msg == nil || msg.Message == nil {
		return "", "", nil
	}
	m := msg.Message
	if m == nil {
		return "", "", nil
	}

	makeDataURL := func(mime string, data []byte) string {
		mime = strings.TrimSpace(mime)
		if mime == "" {
			mime = "application/octet-stream"
		}
		return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	}

	switch {
	case m.GetImageMessage() != nil:
		im := m.GetImageMessage()
		data, err := b.client.Download(context.Background(), im)
		if err != nil {
			return "", "", fmt.Errorf("gagal download image: %w", err)
		}
		return makeDataURL(firstNonEmpty(im.GetMimetype(), "image/jpeg"), data), "image", nil
	case m.GetVideoMessage() != nil:
		return "", "", errors.New("qc media hanya support image + caption")
	case m.GetStickerMessage() != nil:
		sm := m.GetStickerMessage()
		data, err := b.client.Download(context.Background(), sm)
		if err != nil {
			return "", "", fmt.Errorf("gagal download sticker: %w", err)
		}
		return makeDataURL(firstNonEmpty(sm.GetMimetype(), "image/webp"), data), "image", nil
	case m.GetDocumentMessage() != nil:
		dm := m.GetDocumentMessage()
		mime := strings.ToLower(strings.TrimSpace(dm.GetMimetype()))
		if strings.HasPrefix(mime, "image/") {
			data, err := b.client.Download(context.Background(), dm)
			if err != nil {
				return "", "", fmt.Errorf("gagal download document image: %w", err)
			}
			return makeDataURL(firstNonEmpty(dm.GetMimetype(), "image/jpeg"), data), "image", nil
		}
		if strings.HasPrefix(mime, "video/") {
			return "", "", errors.New("qc media hanya support image + caption")
		}
	}
	return "", "", nil
}

func hasMediaInProto(m *waProto.Message) bool {
	if m == nil {
		return false
	}
	if m.GetImageMessage() != nil || m.GetVideoMessage() != nil || m.GetStickerMessage() != nil {
		return true
	}
	if dm := m.GetDocumentMessage(); dm != nil {
		mime := strings.ToLower(strings.TrimSpace(dm.GetMimetype()))
		return strings.HasPrefix(mime, "image/") || strings.HasPrefix(mime, "video/")
	}
	return false
}

func requestQuoteCard(name, avatarURL, text, mediaURL, mediaType, replyName, replyText string) ([]byte, error) {
	type fromPhoto struct {
		URL string `json:"url"`
	}
	type from struct {
		ID        int       `json:"id"`
		FirstName string    `json:"first_name"`
		LastName  string    `json:"last_name"`
		Name      string    `json:"name"`
		Photo     fromPhoto `json:"photo"`
	}
	type media struct {
		URL string `json:"url"`
	}
	type replyMessage struct {
		Name     string   `json:"name"`
		Text     string   `json:"text"`
		Entities []string `json:"entities"`
		ChatID   int      `json:"chatId"`
	}
	type message struct {
		From         from         `json:"from"`
		Text         string       `json:"text"`
		Entities     []string     `json:"entities"`
		Avatar       bool         `json:"avatar"`
		Media        media        `json:"media"`
		MediaType    string       `json:"mediaType"`
		ReplyMessage replyMessage `json:"replyMessage"`
	}
	reqBody := struct {
		Messages        []message `json:"messages"`
		BackgroundColor string    `json:"backgroundColor"`
		Width           int       `json:"width"`
		Height          int       `json:"height"`
		Scale           int       `json:"scale"`
		Type            string    `json:"type"`
		Format          string    `json:"format"`
		EmojiStyle      string    `json:"emojiStyle"`
	}{
		Messages: []message{{
			From: from{
				ID:        1,
				FirstName: name,
				LastName:  "",
				Name:      name,
				Photo:     fromPhoto{URL: avatarURL},
			},
			Text:      text,
			Entities:  []string{},
			Avatar:    true,
			Media:     media{URL: mediaURL},
			MediaType: mediaType,
			ReplyMessage: replyMessage{
				Name:     replyName,
				Text:     replyText,
				Entities: []string{},
				ChatID:   1,
			},
		}},
		BackgroundColor: "#292232",
		Width:           512,
		Height:          512,
		Scale:           2,
		Type:            "quote",
		Format:          "png",
		EmojiStyle:      "apple",
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, qcAPIBase, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "image/png")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("qc api status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	contentType := strings.ToLower(strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0]))
	if contentType != "" && !strings.Contains(contentType, "image/png") {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("qc api content-type %s: %s", contentType, strings.TrimSpace(string(body)))
	}
	out, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, errors.New("qc api mengembalikan body kosong")
	}
	return out, nil
}

func (b *Bot) HandleTikTok(msg *events.Message, inputURL string) (retErr error) {
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	raw := strings.TrimSpace(inputURL)
	if raw == "" {
		return errors.New("url tiktok kosong")
	}
	if !looksLikeTikTokURL(raw) {
		return errors.New("url bukan tiktok")
	}

	info, err := fetchTikWM(raw)
	if err != nil {
		return err
	}

	baseCaption := buildTikTokCaption(info)
	infoText := buildTikTokInfoText(info)

	// Photo mode posts should prefer image array even if "play" contains audio URL.
	if len(info.Data.Images) > 0 {
		for i, imgURL := range info.Data.Images {
			data, mime, err := downloadFromURL(imgURL)
			if err != nil {
				return err
			}
			if mime == "" {
				mime = "image/jpeg"
			}
			imgCaption := ""
			if i == len(info.Data.Images)-1 {
				if baseCaption != "" {
					imgCaption = baseCaption + "\n\n" + infoText
				} else {
					imgCaption = infoText
				}
			}
			if err := b.sendImageFromBytes(msg, data, mime, imgCaption); err != nil {
				return err
			}
		}
		b.sendTikTokMusic(msg, info)
		return nil
	}

	videoURL := firstNonEmpty(info.Data.Play, info.Data.WMPlay)
	if videoURL != "" {
		data, mime, err := downloadFromURL(videoURL)
		if err != nil {
			return err
		}
		if mime == "" {
			mime = "video/mp4"
		}
		videoCaption := baseCaption
		if infoText != "" {
			if videoCaption != "" {
				videoCaption += "\n\n" + infoText
			} else {
				videoCaption = infoText
			}
		}
		if err := b.sendVideoFromBytes(msg, data, mime, videoCaption); err != nil {
			return err
		}
		b.sendTikTokMusic(msg, info)
		return nil
	}

	return errors.New("media tiktok tidak ditemukan")
}

func (b *Bot) HandleInstagram(msg *events.Message, inputURL string) (retErr error) {
	_ = b.react(msg, "⏳")
	defer func() {
		if retErr != nil {
			_ = b.react(msg, "❌")
			return
		}
		_ = b.react(msg, "✅")
	}()

	raw := strings.TrimSpace(inputURL)
	if raw == "" {
		return errors.New("url instagram kosong")
	}
	if !looksLikeInstagramURL(raw) {
		return errors.New("url bukan instagram")
	}

	respData, err := fetchInstagramFastDL(raw)
	if err != nil {
		return err
	}

	var stories []instagramFastDLStoryData
	if err := json.Unmarshal(respData.Data, &stories); err == nil && len(stories) > 0 {
		return b.sendInstagramStoryItems(msg, stories)
	}

	var single instagramFastDLSingleData
	if err := json.Unmarshal(respData.Data, &single); err != nil {
		return fmt.Errorf("format response fastdl tidak dikenali: %w", err)
	}
	if single.Success != nil && !*single.Success {
		msg := strings.TrimSpace(single.Message)
		if msg == "" {
			msg = "konten tidak bisa diunduh (mungkin private)"
		}
		return errors.New(msg)
	}
	if len(single.URL) == 0 {
		msg := strings.TrimSpace(single.Message)
		if msg == "" {
			msg = "media instagram tidak ditemukan"
		}
		return errors.New(msg)
	}

	caption := buildInstagramCaption(single.Meta, len(single.URL))
	for i, item := range single.URL {
		if strings.TrimSpace(item.URL) == "" {
			continue
		}
		data, mime, err := downloadFromURL(item.URL)
		if err != nil {
			return err
		}
		sendCaption := ""
		if i == 0 {
			sendCaption = caption
		}
		if isInstagramVideo(item, mime) {
			if mime == "" {
				mime = "video/mp4"
			}
			if err := b.sendVideoFromBytes(msg, data, mime, sendCaption); err != nil {
				return err
			}
			continue
		}
		if mime == "" {
			mime = "image/jpeg"
		}
		if err := b.sendImageFromBytes(msg, data, mime, sendCaption); err != nil {
			return err
		}
	}
	return nil
}

func getQuotedMessage(msg *events.Message) *waProto.Message {
	if msg == nil || msg.Message == nil {
		return nil
	}
	if etm := msg.Message.GetExtendedTextMessage(); etm != nil && etm.GetContextInfo() != nil {
		return etm.GetContextInfo().GetQuotedMessage()
	}
	if im := msg.Message.GetImageMessage(); im != nil && im.GetContextInfo() != nil {
		return im.GetContextInfo().GetQuotedMessage()
	}
	if vm := msg.Message.GetVideoMessage(); vm != nil && vm.GetContextInfo() != nil {
		return vm.GetContextInfo().GetQuotedMessage()
	}
	if am := msg.Message.GetAudioMessage(); am != nil && am.GetContextInfo() != nil {
		return am.GetContextInfo().GetQuotedMessage()
	}
	if dm := msg.Message.GetDocumentMessage(); dm != nil && dm.GetContextInfo() != nil {
		return dm.GetContextInfo().GetQuotedMessage()
	}
	if sm := msg.Message.GetStickerMessage(); sm != nil && sm.GetContextInfo() != nil {
		return sm.GetContextInfo().GetQuotedMessage()
	}
	return nil
}

func getStickerSourceMessage(msg *events.Message) *waProto.Message {
	if msg == nil {
		return nil
	}
	if quoted := getQuotedMessage(msg); quoted != nil {
		return quoted
	}
	if msg.Message == nil {
		return nil
	}
	if msg.Message.GetImageMessage() != nil || msg.Message.GetVideoMessage() != nil || msg.Message.GetStickerMessage() != nil {
		return msg.Message
	}
	return nil
}

func getToAudioSourceMessage(msg *events.Message) *waProto.Message {
	if msg == nil {
		return nil
	}
	if quoted := getQuotedMessage(msg); quoted != nil {
		return quoted
	}
	if msg.Message == nil {
		return nil
	}
	if msg.Message.GetVideoMessage() != nil || msg.Message.GetAudioMessage() != nil || msg.Message.GetDocumentMessage() != nil {
		return msg.Message
	}
	return nil
}

func getUpscaleSourceMessage(msg *events.Message) *waProto.Message {
	if msg == nil {
		return nil
	}
	if quoted := getQuotedMessage(msg); quoted != nil {
		return quoted
	}
	if msg.Message == nil {
		return nil
	}
	if msg.Message.GetImageMessage() != nil || msg.Message.GetStickerMessage() != nil || msg.Message.GetDocumentMessage() != nil {
		return msg.Message
	}
	return nil
}

func getQuotedParticipant(msg *events.Message) types.JID {
	if msg == nil || msg.Message == nil {
		return types.EmptyJID
	}
	var ci *waProto.ContextInfo
	switch {
	case msg.Message.GetExtendedTextMessage() != nil:
		ci = msg.Message.GetExtendedTextMessage().GetContextInfo()
	case msg.Message.GetImageMessage() != nil:
		ci = msg.Message.GetImageMessage().GetContextInfo()
	case msg.Message.GetVideoMessage() != nil:
		ci = msg.Message.GetVideoMessage().GetContextInfo()
	case msg.Message.GetDocumentMessage() != nil:
		ci = msg.Message.GetDocumentMessage().GetContextInfo()
	case msg.Message.GetAudioMessage() != nil:
		ci = msg.Message.GetAudioMessage().GetContextInfo()
	case msg.Message.GetStickerMessage() != nil:
		ci = msg.Message.GetStickerMessage().GetContextInfo()
	}
	if ci == nil || ci.GetParticipant() == "" {
		return types.EmptyJID
	}
	jid, err := types.ParseJID(ci.GetParticipant())
	if err != nil {
		return types.EmptyJID
	}
	return jid.ToNonAD()
}

func extractTextFromProtoMessage(m *waProto.Message) string {
	if m == nil {
		return ""
	}
	switch {
	case m.GetConversation() != "":
		return m.GetConversation()
	case m.GetExtendedTextMessage() != nil:
		return m.GetExtendedTextMessage().GetText()
	case m.GetImageMessage() != nil:
		return m.GetImageMessage().GetCaption()
	case m.GetVideoMessage() != nil:
		return m.GetVideoMessage().GetCaption()
	case m.GetDocumentMessage() != nil:
		return m.GetDocumentMessage().GetCaption()
	default:
		return ""
	}
}

func downloadQuotedMedia(b *Bot, quoted *waProto.Message, allowed []string) ([]byte, string, string, error) {
	allow := make(map[string]struct{}, len(allowed))
	for _, a := range allowed {
		allow[a] = struct{}{}
	}

	switch {
	case quoted.GetStickerMessage() != nil:
		if _, ok := allow["sticker"]; !ok {
			return nil, "", "", errors.New("media tidak didukung")
		}
		data, err := b.client.Download(context.Background(), quoted.GetStickerMessage())
		return data, ".webp", "sticker", err
	case quoted.GetImageMessage() != nil:
		if _, ok := allow["image"]; !ok {
			return nil, "", "", errors.New("media tidak didukung")
		}
		data, err := b.client.Download(context.Background(), quoted.GetImageMessage())
		ext := extFromMime(quoted.GetImageMessage().GetMimetype(), ".jpg")
		return data, ext, "image", err
	case quoted.GetVideoMessage() != nil:
		if _, ok := allow["video"]; !ok {
			return nil, "", "", errors.New("media tidak didukung")
		}
		data, err := b.client.Download(context.Background(), quoted.GetVideoMessage())
		ext := extFromMime(quoted.GetVideoMessage().GetMimetype(), ".mp4")
		return data, ext, "video", err
	case quoted.GetAudioMessage() != nil:
		if _, ok := allow["audio"]; !ok {
			return nil, "", "", errors.New("media tidak didukung")
		}
		data, err := b.client.Download(context.Background(), quoted.GetAudioMessage())
		ext := extFromMime(quoted.GetAudioMessage().GetMimetype(), ".ogg")
		return data, ext, "audio", err
	case quoted.GetDocumentMessage() != nil:
		mime := quoted.GetDocumentMessage().GetMimetype()
		isAudio := strings.HasPrefix(mime, "audio/")
		isVideo := strings.HasPrefix(mime, "video/")
		if !isAudio && !isVideo {
			return nil, "", "", errors.New("dokumen bukan audio/video")
		}
		if isAudio {
			if _, ok := allow["audio"]; !ok {
				return nil, "", "", errors.New("media tidak didukung")
			}
		}
		if isVideo {
			if _, ok := allow["video"]; !ok {
				return nil, "", "", errors.New("media tidak didukung")
			}
		}
		data, err := b.client.Download(context.Background(), quoted.GetDocumentMessage())
		ext := extFromMime(mime, ".bin")
		kind := "audio"
		if isVideo {
			kind = "video"
		}
		return data, ext, kind, err
	default:
		return nil, "", "", errors.New("reply media dulu")
	}
}

func (b *Bot) sendImageFromBytes(msg *events.Message, data []byte, mime, caption string) error {
	up, err := b.client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	w, h := detectImageSize(data)
	_, err = b.client.SendMessage(context.Background(), msg.Info.Chat, &waProto.Message{
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
		},
	})
	return err
}

func (b *Bot) sendVideoFromBytes(msg *events.Message, data []byte, mime, caption string) error {
	up, err := b.client.Upload(context.Background(), data, whatsmeow.MediaVideo)
	if err != nil {
		return err
	}
	_, err = b.client.SendMessage(context.Background(), msg.Info.Chat, &waProto.Message{
		VideoMessage: &waProto.VideoMessage{
			Caption:       proto.String(caption),
			Mimetype:      proto.String(mime),
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
		},
	})
	return err
}

func (b *Bot) sendStickerFromBytes(msg *events.Message, data []byte, animated bool) error {
	pack, author := b.store.StickerWM()
	if wmData, err := addStickerMetadata(data, pack, author); err == nil {
		data = wmData
	} else {
		log.Printf("gagal inject wm sticker: %v", err)
	}

	up, err := b.client.Upload(context.Background(), data, whatsmeow.MediaImage)
	if err != nil {
		return err
	}
	_, err = b.client.SendMessage(context.Background(), msg.Info.Chat, &waProto.Message{
		StickerMessage: &waProto.StickerMessage{
			Mimetype:      proto.String("image/webp"),
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
			IsAnimated:    proto.Bool(animated),
		},
	})
	return err
}

func (b *Bot) sendAudioFromBytes(msg *events.Message, data []byte, mime string, ptt bool) error {
	up, err := b.client.Upload(context.Background(), data, whatsmeow.MediaAudio)
	if err != nil {
		return err
	}
	_, err = b.client.SendMessage(context.Background(), msg.Info.Chat, &waProto.Message{
		AudioMessage: &waProto.AudioMessage{
			Mimetype:      proto.String(mime),
			URL:           proto.String(up.URL),
			DirectPath:    proto.String(up.DirectPath),
			MediaKey:      up.MediaKey,
			FileEncSHA256: up.FileEncSHA256,
			FileSHA256:    up.FileSHA256,
			FileLength:    proto.Uint64(up.FileLength),
			PTT:           proto.Bool(ptt),
			Seconds:       proto.Uint32(0),
		},
	})
	return err
}

func requireFFmpeg() error {
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		return errors.New("ffmpeg belum terpasang")
	}
	return nil
}

func runFFmpeg(args ...string) error {
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s == "" {
			return err
		}
		return fmt.Errorf("ffmpeg error: %s", s)
	}
	return nil
}

func runFFmpegOutput(args ...string) (string, error) {
	cmd := exec.Command("ffmpeg", args...)
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			return "", err
		}
		return s, fmt.Errorf("ffmpeg error: %s", s)
	}
	return s, nil
}

func convertToAudio(in, out string) error {
	attempts := [][]string{
		{"-y", "-i", in, "-map", "a:0", "-vn", "-ac", "2", "-ar", "44100", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", out},
		{"-y", "-i", in, "-vn", "-ac", "2", "-ar", "44100", "-c:a", "aac", "-b:a", "128k", "-movflags", "+faststart", out},
	}

	var lastErr error
	for i, args := range attempts {
		ffOut, err := runFFmpegOutput(args...)
		if err == nil {
			return nil
		}
		lastErr = err
		if strings.Contains(ffOut, "matches no streams") || strings.Contains(strings.ToLower(ffOut), "does not contain any stream") {
			return errors.New("media tidak punya audio")
		}
		if i == 0 && !strings.Contains(strings.ToLower(ffOut), "matches no streams") {
			break
		}
	}
	return lastErr
}

func convertToVoiceNote(in, out string) error {
	attempts := [][]string{
		{"-y", "-i", in, "-map", "a:0", "-vn", "-ac", "1", "-ar", "48000", "-c:a", "libopus", "-b:a", "64k", "-f", "ogg", out},
		{"-y", "-i", in, "-map", "a:0", "-vn", "-ac", "1", "-ar", "48000", "-c:a", "opus", "-b:a", "64k", "-f", "ogg", out},
		{"-y", "-i", in, "-vn", "-ac", "1", "-ar", "48000", "-c:a", "libopus", "-b:a", "64k", "-f", "ogg", out},
	}

	var lastErr error
	for _, args := range attempts {
		ffOut, err := runFFmpegOutput(args...)
		if err == nil {
			return nil
		}
		lastErr = err
		if strings.Contains(ffOut, "matches no streams") || strings.Contains(strings.ToLower(ffOut), "does not contain any stream") {
			return errors.New("media tidak punya audio")
		}
	}
	return lastErr
}

func convertToWebP(in, out, vf string, isVideo bool) error {
	// Prefer webp tools if available (more reliable on ffmpeg builds without webp encoder).
	if isVideo {
		if _, err := exec.LookPath("gif2webp"); err == nil {
			if err := convertToWebPWithTools(in, out, vf, true); err == nil {
				return nil
			}
		}
	} else {
		if _, err := exec.LookPath("cwebp"); err == nil {
			if err := convertToWebPWithTools(in, out, vf, false); err == nil {
				return nil
			}
		}
	}

	base := []string{"-y", "-i", in}
	if isVideo {
		base = append(base, "-t", "8")
	}

	// Try libwebp first, then fallback to native webp encoder if libwebp isn't available.
	var attempts [][]string
	attempts = append(attempts, append([]string{}, base...))
	attempts[0] = append(attempts[0],
		"-vf", vf,
		"-c:v", "libwebp",
		"-lossless", "1",
		"-q:v", "50",
		"-loop", "0",
		"-an",
		out,
	)

	attempts = append(attempts, append([]string{}, base...))
	attempts[1] = append(attempts[1],
		"-vf", vf,
		"-c:v", "webp",
		"-q:v", "50",
		"-loop", "0",
		"-an",
		out,
	)

	var lastErr error
	for _, args := range attempts {
		if err := runFFmpeg(args...); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	// Final fallback path when ffmpeg is built without webp encoder.
	if fallbackErr := convertToWebPWithTools(in, out, vf, isVideo); fallbackErr == nil {
		return nil
	} else {
		return fmt.Errorf("webp convert gagal (ffmpeg=%v, fallback=%v)", lastErr, fallbackErr)
	}
}

func convertToWebPWithTools(in, out, vf string, isVideo bool) error {
	if isVideo {
		if _, err := exec.LookPath("gif2webp"); err != nil {
			return errors.New("gif2webp tidak ditemukan")
		}
		gifFile := strings.TrimSuffix(out, filepath.Ext(out)) + ".gif"
		if err := runFFmpeg("-y", "-i", in, "-t", "8", "-vf", vf, gifFile); err != nil {
			return err
		}
		defer os.Remove(gifFile)
		return runCmd("gif2webp", "-q", "80", "-mixed", gifFile, "-o", out)
	}

	if _, err := exec.LookPath("cwebp"); err != nil {
		return errors.New("cwebp tidak ditemukan")
	}
	pngFile := strings.TrimSuffix(out, filepath.Ext(out)) + ".png"
	if err := runFFmpeg("-y", "-i", in, "-frames:v", "1", "-vf", vf, pngFile); err != nil {
		return err
	}
	defer os.Remove(pngFile)
	return runCmd("cwebp", "-q", "80", pngFile, "-o", out)
}

func runCmd(bin string, args ...string) error {
	cmd := exec.Command(bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		s := strings.TrimSpace(string(out))
		if s == "" {
			return err
		}
		return fmt.Errorf("%s error: %s", bin, s)
	}
	return nil
}

func makeTempFiles(inExt, outExt string) (string, string, func(), error) {
	dir, err := os.MkdirTemp("", "meow-media-")
	if err != nil {
		return "", "", nil, err
	}
	in := filepath.Join(dir, "input"+normalizeExt(inExt))
	out := filepath.Join(dir, "output"+normalizeExt(outExt))
	cleanup := func() { _ = os.RemoveAll(dir) }
	return in, out, cleanup, nil
}

func normalizeExt(ext string) string {
	ext = strings.TrimSpace(ext)
	if ext == "" {
		return ".bin"
	}
	if !strings.HasPrefix(ext, ".") {
		return "." + ext
	}
	return ext
}

func extFromMime(mime, fallback string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch {
	case strings.Contains(mime, "jpeg"):
		return ".jpg"
	case strings.Contains(mime, "png"):
		return ".png"
	case strings.Contains(mime, "webp"):
		return ".webp"
	case strings.Contains(mime, "gif"):
		return ".gif"
	case strings.Contains(mime, "mp4"):
		return ".mp4"
	case strings.Contains(mime, "webm"):
		return ".webm"
	case strings.Contains(mime, "ogg"):
		return ".ogg"
	case strings.Contains(mime, "mpeg") || strings.Contains(mime, "mp3"):
		return ".mp3"
	default:
		return fallback
	}
}

func addStickerMetadata(webp []byte, pack, author string) ([]byte, error) {
	pack = strings.TrimSpace(pack)
	author = strings.TrimSpace(author)
	if pack == "" {
		pack = "meow bot"
	}
	if author == "" {
		author = "meow"
	}
	exif, err := buildStickerEXIF(pack, author)
	if err != nil {
		return nil, err
	}
	if _, err := exec.LookPath("webpmux"); err != nil {
		return webp, fmt.Errorf("webpmux tidak ditemukan, wm sticker diskip")
	}

	dir, err := os.MkdirTemp("", "meow-wm-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "in.webp")
	exifFile := filepath.Join(dir, "meta.exif")
	out := filepath.Join(dir, "out.webp")

	if err := os.WriteFile(in, webp, 0o600); err != nil {
		return nil, err
	}
	if err := os.WriteFile(exifFile, exif, 0o600); err != nil {
		return nil, err
	}
	if err := runCmd("webpmux", "-set", "exif", exifFile, in, "-o", out); err != nil {
		return nil, err
	}
	updated, err := os.ReadFile(out)
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func buildStickerEXIF(pack, author string) ([]byte, error) {
	meta := map[string]any{
		"sticker-pack-id":        "meow.bot",
		"sticker-pack-name":      pack,
		"sticker-pack-publisher": author,
		"emojis":                 []string{""},
	}
	js, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	exif := []byte{
		0x49, 0x49, 0x2A, 0x00,
		0x08, 0x00, 0x00, 0x00,
		0x01, 0x00,
		0x41, 0x57,
		0x07, 0x00,
	}
	sz := uint32ToLE(uint32(len(js)))
	exif = append(exif, sz...)
	exif = append(exif, 0x16, 0x00, 0x00, 0x00)
	exif = append(exif, js...)
	return exif, nil
}

func uint32ToLE(v uint32) []byte {
	return []byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)}
}

func looksLikeTikTokURL(raw string) bool {
	u, err := neturl.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	h := strings.ToLower(u.Host)
	return strings.Contains(h, "tiktok.com") || strings.Contains(h, "vt.tiktok.com")
}

func looksLikeInstagramURL(raw string) bool {
	u, err := neturl.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	h := strings.ToLower(u.Host)
	return strings.Contains(h, "instagram.com")
}

func fetchTikWM(tiktokURL string) (*tikwmResponse, error) {
	q := neturl.Values{}
	q.Set("url", tiktokURL)
	endpoint := tikwmAPIBase + "?" + q.Encode()

	client := &http.Client{Timeout: 45 * time.Second}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("tikwm status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out tikwmResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if out.Code != 0 {
		msg := strings.TrimSpace(out.Msg)
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("tikwm error: %s", msg)
	}
	return &out, nil
}

func fetchInstagramFastDL(instagramURL string) (*instagramFastDLResponse, error) {
	u, err := neturl.Parse(instagramFastDLAPIBase)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("url", instagramURL)
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("fastdl status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out instagramFastDLResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if !out.Status {
		return nil, errors.New("fastdl gagal memproses url")
	}
	return &out, nil
}

func downloadFromURL(raw string) ([]byte, string, error) {
	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("download status %d", resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	mime := strings.TrimSpace(strings.Split(resp.Header.Get("Content-Type"), ";")[0])
	return data, mime, nil
}

func buildTikTokCaption(info *tikwmResponse) string {
	if info == nil {
		return "🎬 TikTok Downloader"
	}
	title := strings.TrimSpace(info.Data.Title)
	user := strings.TrimSpace(firstNonEmpty(info.Data.Author.UniqueID, info.Data.Author.Nickname))
	if len(title) > 180 {
		title = title[:180] + "..."
	}
	switch {
	case user != "" && title != "":
		return fmt.Sprintf("🎬 TikTok @%s\n📝 %s", user, title)
	case user != "":
		return fmt.Sprintf("🎬 TikTok @%s", user)
	case title != "":
		return "🎬 TikTok\n📝 " + title
	default:
		return "🎬 TikTok Downloader"
	}
}

func (b *Bot) resolveDisplayName(jid types.JID) string {
	if b == nil || b.client == nil || b.client.Store == nil || b.client.Store.Contacts == nil || jid.IsEmpty() {
		return ""
	}
	ci, err := b.client.Store.Contacts.GetContact(context.Background(), jid.ToNonAD())
	if err != nil {
		return ""
	}
	switch {
	case strings.TrimSpace(ci.PushName) != "":
		return strings.TrimSpace(ci.PushName)
	case strings.TrimSpace(ci.FullName) != "":
		return strings.TrimSpace(ci.FullName)
	case strings.TrimSpace(ci.FirstName) != "":
		return strings.TrimSpace(ci.FirstName)
	default:
		return ""
	}
}

func (b *Bot) prepareAvatarFile(jid types.JID, out string) error {
	info, err := b.client.GetProfilePictureInfo(context.Background(), jid.ToNonAD(), &whatsmeow.GetProfilePictureParams{Preview: true})
	if err == nil && info != nil && strings.TrimSpace(info.URL) != "" {
		data, _, err := downloadFromURL(info.URL)
		if err == nil && len(data) > 0 {
			return os.WriteFile(out, data, 0o600)
		}
	}
	// Fallback avatar when profile photo is unavailable/private.
	return runFFmpeg("-y", "-f", "lavfi", "-i", "color=c=0x334155:s=256x256:d=1", "-frames:v", "1", out)
}

func renderQCImage(avatarPath, name, text, out string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Unknown"
	}
	text = strings.TrimSpace(text)
	if text == "" {
		text = "..."
	}
	wrapped := wrapQuoteText(text, 28, 6)
	nameEsc := escapeDrawText(name)
	textEsc := escapeDrawText(wrapped)

	fontOpt := ""
	if ff, ok := findFontFile(); ok {
		fontOpt = "fontfile='" + escapeDrawText(ff) + "':"
	}

	filter := "" +
		"[1:v]scale=120:120[av];" +
		"[0:v]drawbox=x=36:y=36:w=648:h=648:color=0x111827@0.97:t=fill[card];" +
		"[card][av]overlay=72:80[tmp1];" +
		"[tmp1]drawtext=" + fontOpt + "text='" + nameEsc + "':x=220:y=102:fontsize=34:fontcolor=white[tmp2];" +
		"[tmp2]drawtext=" + fontOpt + "text='" + textEsc + "':x=72:y=250:fontsize=30:line_spacing=10:fontcolor=white"

	return runFFmpeg(
		"-y",
		"-f", "lavfi", "-i", "color=c=0x0f172a:s=720x720:d=1",
		"-i", avatarPath,
		"-filter_complex", filter,
		"-frames:v", "1",
		out,
	)
}

func findFontFile() (string, bool) {
	candidates := []string{
		"/System/Library/Fonts/Supplemental/Arial Unicode.ttf",
		"/System/Library/Fonts/Supplemental/Arial.ttf",
		"/Library/Fonts/Arial Unicode.ttf",
		"/Library/Fonts/Arial.ttf",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p, true
		}
	}
	return "", false
}

func wrapQuoteText(text string, maxCols, maxLines int) string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return ""
	}
	lines := make([]string, 0, maxLines)
	cur := ""
	for _, w := range words {
		if len(cur)+len(w)+1 <= maxCols {
			if cur == "" {
				cur = w
			} else {
				cur += " " + w
			}
			continue
		}
		lines = append(lines, cur)
		cur = w
		if len(lines) >= maxLines {
			break
		}
	}
	if len(lines) < maxLines && cur != "" {
		lines = append(lines, cur)
	}
	if len(lines) > maxLines {
		lines = lines[:maxLines]
	}
	if len(lines) == maxLines && len(words) > 0 {
		last := lines[maxLines-1]
		if !strings.HasSuffix(last, "...") {
			if len(last) > maxCols-3 {
				last = last[:maxCols-3]
			}
			lines[maxLines-1] = strings.TrimSpace(last) + "..."
		}
	}
	return strings.Join(lines, "\n")
}

func escapeDrawText(s string) string {
	s = html.UnescapeString(s)
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ":", "\\:")
	s = strings.ReplaceAll(s, "'", "\\'")
	s = strings.ReplaceAll(s, "%", "\\%")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

func buildTikTokInfoText(info *tikwmResponse) string {
	if info == nil {
		return "🎬 TikTok info tidak tersedia"
	}
	user := strings.TrimSpace(firstNonEmpty(info.Data.Author.UniqueID, info.Data.Author.Nickname))
	nick := strings.TrimSpace(info.Data.Author.Nickname)
	title := strings.TrimSpace(info.Data.Title)
	if len(title) > 220 {
		title = title[:220] + "..."
	}
	date := "-"
	if info.Data.CreateTime > 0 {
		date = time.Unix(info.Data.CreateTime, 0).Format("2006-01-02 15:04")
	}

	return "" +
		"📌 *TikTok Info*\n" +
		fmt.Sprintf("👤 User: @%s", fallbackText(user, "-")) + "\n" +
		fmt.Sprintf("🏷️ Nick: %s", fallbackText(nick, "-")) + "\n" +
		fmt.Sprintf("🆔 ID: %s", fallbackText(info.Data.ID, "-")) + "\n" +
		fmt.Sprintf("🌍 Region: %s", fallbackText(info.Data.Region, "-")) + "\n" +
		fmt.Sprintf("⏱️ Durasi: %ds", info.Data.Duration) + "\n" +
		fmt.Sprintf("📅 Upload: %s", date) + "\n" +
		fmt.Sprintf("▶️ %s  ❤️ %s  💬 %s  🔁 %s", formatCount(info.Data.PlayCount), formatCount(info.Data.DiggCount), formatCount(info.Data.CommentCount), formatCount(info.Data.ShareCount)) + "\n" +
		"📝 " + fallbackText(title, "-")
}

func (b *Bot) sendTikTokMusic(msg *events.Message, info *tikwmResponse) {
	if info == nil {
		return
	}
	musicURL := firstNonEmpty(info.Data.Music, info.Data.MusicInfo.Play)
	if musicURL == "" {
		return
	}
	data, mime, err := downloadFromURL(musicURL)
	if err != nil {
		log.Printf("gagal download music tiktok: %v", err)
		return
	}
	if mime == "" {
		mime = "audio/mpeg"
	}
	if err := b.sendAudioFromBytes(msg, data, mime, false); err != nil {
		log.Printf("gagal kirim music tiktok: %v", err)
	}
}

func formatCount(v int64) string {
	if v < 1000 {
		return strconv.FormatInt(v, 10)
	}
	if v < 1000000 {
		return fmt.Sprintf("%.1fK", float64(v)/1000)
	}
	return fmt.Sprintf("%.1fM", float64(v)/1000000)
}

func fallbackText(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		v = strings.TrimSpace(v)
		if v != "" {
			return v
		}
	}
	return ""
}

func buildInstagramCaption(meta instagramFastDLMeta, total int) string {
	user := strings.TrimSpace(meta.Username)
	title := strings.TrimSpace(meta.Title)
	source := strings.TrimSpace(meta.Source)
	if len(title) > 220 {
		title = title[:220] + "..."
	}
	date := "-"
	if meta.TakenAt > 0 {
		date = time.Unix(meta.TakenAt, 0).Format("2006-01-02 15:04")
	}

	base := "📥 *Instagram Downloader*"
	if total > 1 {
		base += fmt.Sprintf("\n📦 Total media: %d", total)
	}
	base += "\n👤 User: @" + fallbackText(user, "-")
	base += "\n📅 Upload: " + date
	if source != "" {
		base += "\n🔗 Source: " + source
	}
	if title != "" {
		base += "\n📝 " + title
	}
	return base
}

func isInstagramVideo(item instagramFastDLMedia, mime string) bool {
	typeHint := strings.ToLower(strings.TrimSpace(item.Type))
	extHint := strings.ToLower(strings.TrimSpace(item.Ext))
	urlHint := strings.ToLower(strings.TrimSpace(item.URL))
	mime = strings.ToLower(strings.TrimSpace(mime))
	if strings.Contains(typeHint, "mp4") || strings.Contains(typeHint, "video") {
		return true
	}
	if extHint == "mp4" || extHint == "mov" || extHint == "webm" || extHint == "m4v" {
		return true
	}
	if strings.Contains(urlHint, ".mp4") || strings.Contains(urlHint, ".mov") || strings.Contains(urlHint, ".webm") || strings.Contains(urlHint, ".m4v") {
		return true
	}
	return strings.HasPrefix(mime, "video/")
}

func (b *Bot) sendInstagramStoryItems(msg *events.Message, items []instagramFastDLStoryData) error {
	if len(items) == 0 {
		return errors.New("story tidak ditemukan")
	}
	totalMedia := 0
	for _, it := range items {
		totalMedia += len(it.URL)
	}
	for idx, story := range items {
		caption := ""
		if idx == 0 {
			caption = buildInstagramCaption(story.Meta, totalMedia)
		}
		for _, media := range story.URL {
			if strings.TrimSpace(media.URL) == "" {
				continue
			}
			data, mime, err := downloadFromURL(media.URL)
			if err != nil {
				return err
			}
			if isInstagramVideo(media, mime) {
				if mime == "" {
					mime = "video/mp4"
				}
				if err := b.sendVideoFromBytes(msg, data, mime, caption); err != nil {
					return err
				}
			} else {
				if mime == "" {
					mime = "image/jpeg"
				}
				if err := b.sendImageFromBytes(msg, data, mime, caption); err != nil {
					return err
				}
			}
			caption = ""
		}
	}
	return nil
}
