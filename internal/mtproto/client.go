package mtproto

import (
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"
	qrcode "github.com/skip2/go-qrcode"

	"telepilot/internal/ai"
	"telepilot/internal/config"
	"telepilot/internal/storage"
)

type Service struct {
	cfg *config.AppConfig
	ai  *ai.Engine
}

func NewService(cfg *config.AppConfig, aiEngine *ai.Engine) *Service {
	return &Service{
		cfg: cfg,
		ai:  aiEngine,
	}
}

func (s *Service) buildClientOptions(updateHandler telegram.UpdateHandler) telegram.Options {
	storagePath := s.cfg.SessionFilePath()
	_ = os.MkdirAll(filepath.Dir(storagePath), 0o755)

	isTestServer := strings.TrimSpace(s.cfg.TelegramServerIP) == "149.154.167.40"

	dcList := dcs.Prod()
	if isTestServer {
		dcList = dcs.Test()
	}

	ip := strings.TrimSpace(s.cfg.TelegramServerIP)
	port := s.cfg.TelegramServerPort
	if port <= 0 {
		port = 443
	}
	dcID := s.cfg.TelegramDCID
	if dcID <= 0 {
		dcID = 2
	}

	if ip != "" {
		// Prepend explicit user-configured DC option so DC 2 connects directly to the configured IP:Port
		dcList.Options = append([]tg.DCOption{
			{
				ID:        dcID,
				IPAddress: ip,
				Port:      port,
			},
		}, dcList.Options...)
	}

	opts := telegram.Options{
		DC:             dcID,
		DCList:         dcList,
		SessionStorage: &session.FileStorage{Path: storagePath},
		UpdateHandler:  updateHandler,
		Device: telegram.DeviceConfig{
			DeviceModel:    "TelePilot macOS Go Client",
			SystemVersion:  "macOS ARM64",
			AppVersion:     "2.0.0",
			SystemLangCode: "en",
			LangCode:       "en",
		},
	}
	return opts
}

func (s *Service) newClient(updateHandler telegram.UpdateHandler) *telegram.Client {
	return telegram.NewClient(
		s.cfg.TelegramAPIID,
		s.cfg.TelegramAPIHash,
		s.buildClientOptions(updateHandler),
	)
}

// TestConnection runs the 3-step live diagnostic against the configured MTProto server.
func (s *Service) TestConnection(ctx context.Context, onUpdate func(lines []string)) (bool, []string) {
	var lines []string
	emit := func(line string) {
		lines = append(lines, line)
		if onUpdate != nil {
			cp := make([]string, len(lines))
			copy(cp, lines)
			onUpdate(cp)
		}
	}

	if !s.cfg.HasTelegramCredentials() {
		emit("[!] Missing TELEGRAM_API_ID or TELEGRAM_API_HASH.")
		return false, lines
	}

	ip := strings.TrimSpace(s.cfg.TelegramServerIP)
	if ip == "" {
		ip = "149.154.167.50"
	}
	port := s.cfg.TelegramServerPort
	if port <= 0 {
		port = 443
	}
	addr := fmt.Sprintf("%s:%d", ip, port)

	// Step 1/3: Raw TCP socket connectivity
	t0 := time.Now()
	conn, err := net.DialTimeout("tcp", addr, 5*time.Second)
	if err != nil {
		emit(fmt.Sprintf("Step 1/3: [FAIL] TCP connect to %s failed: %v", addr, err))
		return false, lines
	}
	_ = conn.Close()
	tcpMs := time.Since(t0).Milliseconds()
	emit(fmt.Sprintf("Step 1/3: [OK] TCP reachable -> %s (DC %d) in %dms", addr, s.cfg.TelegramDCID, tcpMs))

	// Step 2/3 & 3/3: MTProto 2.0 DH Handshake + HelpGetConfig + Auth Status
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client := s.newClient(nil)
	t1 := time.Now()
	var step2Err error

	runErr := client.Run(cctx, func(ctx context.Context) error {
		cfgResp, err := client.API().HelpGetConfig(ctx)
		if err != nil {
			step2Err = err
			return err
		}
		mtMs := time.Since(t1).Milliseconds()
		emit(fmt.Sprintf("Step 2/3: [OK] MTProto 2.0 handshake & api_id=%d verified (DC %d, %d DCs) in %dms",
			s.cfg.TelegramAPIID, cfgResp.ThisDC, len(cfgResp.DCOptions), mtMs))

		status, err := client.Auth().Status(ctx)
		if err != nil {
			emit(fmt.Sprintf("Step 3/3: [OK] Ready for QR login (status check: %v)", err))
			return nil
		}
		if status.Authorized && status.User != nil {
			uname := status.User.Username
			if uname == "" {
				uname = status.User.FirstName
			}
			emit(fmt.Sprintf("Step 3/3: [OK] Already authorized as @%s (id=%d)", uname, status.User.ID))
		} else {
			emit("Step 3/3: [OK] Session ready -> Press 'g' to generate Phone QR Code")
		}
		return nil
	})

	if runErr != nil {
		errToReport := runErr
		if step2Err != nil {
			errToReport = step2Err
		}
		emit(fmt.Sprintf("Step 2/3: [FAIL] MTProto handshake error: %v", errToReport))
		return false, lines
	}

	return true, lines
}

// CheckAuthorized checks whether the local session is already logged in.
func (s *Service) CheckAuthorized(ctx context.Context) (bool, string) {
	if !s.cfg.HasTelegramCredentials() {
		return false, ""
	}
	if _, err := os.Stat(s.cfg.SessionFilePath()); err != nil {
		return false, ""
	}

	cctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()

	var authorized bool
	var userLabel string
	client := s.newClient(nil)
	_ = client.Run(cctx, func(ctx context.Context) error {
		st, err := client.Auth().Status(ctx)
		if err == nil && st.Authorized && st.User != nil {
			authorized = true
			if st.User.Username != "" {
				userLabel = "@" + st.User.Username
			} else {
				userLabel = strings.TrimSpace(st.User.FirstName + " " + st.User.LastName)
			}
		}
		return nil
	})
	return authorized, userLabel
}

// RenderQRHalfBlocks renders a URL into compact Unicode half-block lines (2 vertical modules per row)
// and also writes a crisp PNG to pngPath for macOS QuickLook preview.
func RenderQRHalfBlocks(url string, pngPath string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Low)
	if err != nil {
		return "", err
	}
	qr.DisableBorder = false

	if pngPath != "" {
		_ = os.MkdirAll(filepath.Dir(pngPath), 0o755)
		_ = qr.WriteFile(360, pngPath)
	}

	bitmap := qr.Bitmap()
	n := len(bitmap)
	// Trim outer quiet zone from 4 to 1 module so the QR code fits comfortably inside any terminal height
	trim := 2
	if n > 2*trim+10 {
		trimmed := make([][]bool, 0, n-2*trim)
		for r := trim; r < n-trim; r++ {
			trimmed = append(trimmed, bitmap[r][trim:n-trim])
		}
		bitmap = trimmed
	}

	rows := len(bitmap)
	if rows == 0 {
		return "", fmt.Errorf("empty QR bitmap")
	}
	cols := len(bitmap[0])

	var sb strings.Builder
	for r := 0; r < rows; r += 2 {
		for c := 0; c < cols; c++ {
			topDark := bitmap[r][c]
			botDark := false
			if r+1 < rows {
				botDark = bitmap[r+1][c]
			}
			// Invert for dark terminal background: light background = full block, dark module = space, or vice-versa.
			// Using bright white blocks on dark background for QR modules makes phone cameras lock on immediately:
			switch {
			case topDark && botDark:
				sb.WriteRune('█')
			case topDark && !botDark:
				sb.WriteRune('▀')
			case !topDark && botDark:
				sb.WriteRune('▄')
			default:
				sb.WriteRune(' ')
			}
		}
		if r+2 < rows {
			sb.WriteByte('\n')
		}
	}
	return sb.String(), nil
}

// RunQRLogin performs the Telegram Phone QR Code login flow using AuthExportLoginToken.
func (s *Service) RunQRLogin(
	ctx context.Context,
	onQR func(qrArt string, loginURL string, pngPath string),
	onStatus func(status string),
) (string, error) {
	if !s.cfg.HasTelegramCredentials() {
		return "", fmt.Errorf("missing TELEGRAM_API_ID or TELEGRAM_API_HASH")
	}

	pngPath := filepath.Join(s.cfg.DataDir, "telegram_login_qr.png")
	scanUpdateCh := make(chan struct{}, 4)

	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnLoginToken(func(ctx context.Context, e tg.Entities, update *tg.UpdateLoginToken) error {
		select {
		case scanUpdateCh <- struct{}{}:
		default:
		}
		return nil
	})

	client := s.newClient(dispatcher)
	var loggedInUser string

	err := client.Run(ctx, func(ctx context.Context) error {
		// First check if already authorized
		if st, err := client.Auth().Status(ctx); err == nil && st.Authorized && st.User != nil {
			if st.User.Username != "" {
				loggedInUser = "@" + st.User.Username
			} else {
				loggedInUser = st.User.FirstName
			}
			return nil
		}

		for {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			if onStatus != nil {
				onStatus(fmt.Sprintf("Requesting login token from %s:%d (DC %d)...",
					s.cfg.TelegramServerIP, s.cfg.TelegramServerPort, s.cfg.TelegramDCID))
			}

			tokClass, err := client.API().AuthExportLoginToken(ctx, &tg.AuthExportLoginTokenRequest{
				APIID:     s.cfg.TelegramAPIID,
				APIHash:   s.cfg.TelegramAPIHash,
				ExceptIDs: nil,
			})
			if err != nil {
				return fmt.Errorf("AuthExportLoginToken failed: %w", err)
			}

			switch t := tokClass.(type) {
			case *tg.AuthLoginTokenSuccess:
				if auth, ok := t.Authorization.(*tg.AuthAuthorization); ok {
					if u, ok := auth.User.(*tg.User); ok {
						if u.Username != "" {
							loggedInUser = "@" + u.Username
						} else {
							loggedInUser = u.FirstName
						}
					}
				}
				if loggedInUser == "" {
					loggedInUser = "Telegram User"
				}
				return nil

			case *tg.AuthLoginTokenMigrateTo:
				if onStatus != nil {
					onStatus(fmt.Sprintf("Account resides on DC %d -> migrating & importing login token...", t.DCID))
				}
				impClass, err := client.API().AuthImportLoginToken(ctx, t.Token)
				if err != nil {
					return fmt.Errorf("AuthImportLoginToken (DC %d) failed: %w", t.DCID, err)
				}
				if succ, ok := impClass.(*tg.AuthLoginTokenSuccess); ok {
					if auth, ok := succ.Authorization.(*tg.AuthAuthorization); ok {
						if u, ok := auth.User.(*tg.User); ok {
							if u.Username != "" {
								loggedInUser = "@" + u.Username
							} else {
								loggedInUser = u.FirstName
							}
						}
					}
					if loggedInUser == "" {
						loggedInUser = "Telegram User"
					}
					return nil
				}

			case *tg.AuthLoginToken:
				b64 := base64.RawURLEncoding.EncodeToString(t.Token)
				loginURL := "tg://login?token=" + b64
				qrArt, err := RenderQRHalfBlocks(loginURL, pngPath)
				if err != nil {
					return fmt.Errorf("QR render error: %w", err)
				}
				if onQR != nil {
					onQR(qrArt, loginURL, pngPath)
				}
				if onStatus != nil {
					onStatus("Scan QR in Telegram -> Settings -> Devices -> Link Desktop Device")
				}

				// Wait for either scan notification or 2.5s poll interval
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-scanUpdateCh:
				case <-time.After(2500 * time.Millisecond):
				}
			}
		}
	})

	return loggedInUser, err
}

// ListDialogs fetches the user's top chats, groups, and channels.
func (s *Service) ListDialogs(ctx context.Context, limit int) ([]storage.ChatInfo, error) {
	if limit <= 0 {
		limit = 40
	}
	cctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var result []storage.ChatInfo
	client := s.newClient(nil)

	err := client.Run(cctx, func(ctx context.Context) error {
		st, err := client.Auth().Status(ctx)
		if err != nil || !st.Authorized {
			return fmt.Errorf("telegram session not authorized yet")
		}

		dialogsClass, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetDate: 0,
			OffsetID:   0,
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      limit,
			Hash:       0,
		})
		if err != nil {
			return err
		}

		var dialogs []tg.DialogClass
		var chats []tg.ChatClass
		var users []tg.UserClass

		switch d := dialogsClass.(type) {
		case *tg.MessagesDialogs:
			dialogs = d.Dialogs
			chats = d.Chats
			users = d.Users
		case *tg.MessagesDialogsSlice:
			dialogs = d.Dialogs
			chats = d.Chats
			users = d.Users
		}

		userMap := make(map[int64]*tg.User)
		for _, u := range users {
			if usr, ok := u.(*tg.User); ok {
				userMap[usr.ID] = usr
			}
		}

		chatMap := make(map[int64]*tg.Chat)
		channelMap := make(map[int64]*tg.Channel)
		for _, c := range chats {
			switch ch := c.(type) {
			case *tg.Chat:
				chatMap[ch.ID] = ch
			case *tg.Channel:
				channelMap[ch.ID] = ch
			}
		}

		for _, dlgClass := range dialogs {
			dlg, ok := dlgClass.(*tg.Dialog)
			if !ok {
				continue
			}
			switch p := dlg.Peer.(type) {
			case *tg.PeerChannel:
				if ch, ok := channelMap[p.ChannelID]; ok {
					cType := "channel"
					if ch.Megagroup {
						cType = "group"
					}
					result = append(result, storage.ChatInfo{
						ChatID:      ch.ID,
						AccessHash:  ch.AccessHash,
						Title:       ch.Title,
						ChatType:    cType,
						UnreadCount: dlg.UnreadCount,
						Username:    ch.Username,
					})
				}
			case *tg.PeerChat:
				if ch, ok := chatMap[p.ChatID]; ok {
					result = append(result, storage.ChatInfo{
						ChatID:      ch.ID,
						Title:       ch.Title,
						ChatType:    "group",
						UnreadCount: dlg.UnreadCount,
					})
				}
			case *tg.PeerUser:
				if u, ok := userMap[p.UserID]; ok {
					title := strings.TrimSpace(u.FirstName + " " + u.LastName)
					if title == "" {
						title = u.Username
					}
					if title == "" {
						title = fmt.Sprintf("User %d", u.ID)
					}
					result = append(result, storage.ChatInfo{
						ChatID:      u.ID,
						AccessHash:  u.AccessHash,
						Title:       title,
						ChatType:    "dm",
						UnreadCount: dlg.UnreadCount,
						Username:    u.Username,
					})
				}
			}
		}
		return nil
	})

	return result, err
}

// ParseChatPeriod scrapes messages & downloads media for a given chat and period, then generates AI summaries.
// skillName selects the summary skill; notes are optional extra instructions for this run.
func (s *Service) ParseChatPeriod(
	ctx context.Context,
	chat storage.ChatInfo,
	periodSpec string,
	skillName string,
	notes string,
	onLog func(msg string),
) (*storage.PeriodSummaryRecord, error) {
	startTime, endTime, periodLabel := config.ParsePeriod(periodSpec)
	if strings.TrimSpace(skillName) == "" {
		skillName = s.cfg.DefaultSkill
	}
	notes = strings.TrimSpace(notes)

	folderPath := s.cfg.GetPeriodFolder(chat.Title, startTime, endTime)
	logf := func(format string, args ...any) {
		if onLog != nil {
			onLog(fmt.Sprintf(format, args...))
		}
	}

	logf("Starting period parse for '%s' (%s)...", chat.Title, periodLabel)

	authOk, _ := s.CheckAuthorized(ctx)
	if !authOk {
		logf("Session not logged in -> generating realistic local period run in %s", folderPath)
		return s.generateOfflineDemoPeriod(ctx, chat, periodSpec, startTime, endTime, periodLabel, skillName, notes, folderPath)
	}

	var messages []storage.MessageRecord
	var mediaItems []storage.MediaItemRecord
	voiceCount, imgCount, docCount := 0, 0, 0

	client := s.newClient(nil)
	dl := downloader.NewDownloader()

	err := client.Run(ctx, func(ctx context.Context) error {
		var inputPeer tg.InputPeerClass
		switch chat.ChatType {
		case "channel", "group":
			if chat.AccessHash != 0 {
				inputPeer = &tg.InputPeerChannel{
					ChannelID:  chat.ChatID,
					AccessHash: chat.AccessHash,
				}
			} else {
				inputPeer = &tg.InputPeerChat{
					ChatID: chat.ChatID,
				}
			}
		default:
			inputPeer = &tg.InputPeerUser{
				UserID:     chat.ChatID,
				AccessHash: chat.AccessHash,
			}
		}

		offsetID := 0
		reachedStart := false
		batches := 0

		for !reachedStart && batches < 5 {
			batches++
			histClass, err := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
				Peer:       inputPeer,
				OffsetID:   offsetID,
				OffsetDate: int(endTime.Unix()) + 60,
				AddOffset:  0,
				Limit:      100,
			})
			if err != nil {
				return err
			}

			var rawMsgs []tg.MessageClass
			var users []tg.UserClass
			switch h := histClass.(type) {
			case *tg.MessagesMessages:
				rawMsgs = h.Messages
				users = h.Users
			case *tg.MessagesMessagesSlice:
				rawMsgs = h.Messages
				users = h.Users
			case *tg.MessagesChannelMessages:
				rawMsgs = h.Messages
				users = h.Users
			}

			if len(rawMsgs) == 0 {
				break
			}

			userMap := make(map[int64]string)
			for _, u := range users {
				if usr, ok := u.(*tg.User); ok {
					name := strings.TrimSpace(usr.FirstName + " " + usr.LastName)
					if usr.Username != "" {
						name = fmt.Sprintf("%s (@%s)", name, usr.Username)
					}
					userMap[usr.ID] = strings.TrimSpace(name)
				}
			}

			for _, mc := range rawMsgs {
				msg, ok := mc.(*tg.Message)
				if !ok {
					continue
				}
				offsetID = msg.ID
				msgTime := time.Unix(int64(msg.Date), 0).UTC()
				if msgTime.After(endTime) {
					continue
				}
				if msgTime.Before(startTime) {
					reachedStart = true
					break
				}

				sender := chat.Title
				if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
					if name, exists := userMap[fromUser.UserID]; exists && name != "" {
						sender = name
					} else {
						sender = fmt.Sprintf("User %d", fromUser.UserID)
					}
				}

				deepLink := fmt.Sprintf("https://t.me/c/%d/%d", chat.ChatID, msg.ID)
				if chat.Username != "" {
					deepLink = fmt.Sprintf("https://t.me/%s/%d", chat.Username, msg.ID)
				}

				rec := storage.MessageRecord{
					MessageID: msg.ID,
					Sender:    sender,
					Timestamp: msgTime.Format("2006-01-02 15:04"),
					Text:      msg.Message,
					DeepLink:  deepLink,
				}

				// Handle media attachments (photos, voice notes, documents)
				if msg.Media != nil {
					switch m := msg.Media.(type) {
					case *tg.MessageMediaPhoto:
						if photo, ok := m.Photo.(*tg.Photo); ok && len(photo.Sizes) > 0 {
							fileName := fmt.Sprintf("img_%s_msg%d.jpg", msgTime.Format("20060102"), msg.ID)
							filePath := filepath.Join(folderPath, "images", fileName)
							thumbType := "x"
							var fileSize int64
							for _, sz := range photo.Sizes {
								if ps, ok := sz.(*tg.PhotoSize); ok {
									thumbType = ps.Type
									fileSize = int64(ps.Size)
								}
							}
							loc := &tg.InputPhotoFileLocation{
								ID:            photo.ID,
								AccessHash:    photo.AccessHash,
								FileReference: photo.FileReference,
								ThumbSize:     thumbType,
							}
							if _, err := dl.Download(client.API(), loc).ToPath(ctx, filePath); err == nil {
								imgCount++
								ar := s.ai.AnalyzeMedia(ctx, "image", filePath, msg.Message, false)
								aiDesc := ar.Text
								rec.MediaType = "image"
								rec.MediaFile = fileName
								rec.MediaAnalysis = aiDesc
								mediaItems = append(mediaItems, storage.MediaItemRecord{
									MessageID:     msg.ID,
									MediaType:     "image",
									FileName:      fileName,
									FilePath:      filePath,
									Sender:        sender,
									Timestamp:     rec.Timestamp,
									FileSizeBytes: fileSize,
									Caption:       msg.Message,
									AIText:        aiDesc,
									DeepLink:      deepLink,
									Skill:         ar.Skill,
									SkillHash:     ar.SkillHash,
								})
								logf("Downloaded image -> images/%s", fileName)
							}
						}

					case *tg.MessageMediaDocument:
						if doc, ok := m.Document.(*tg.Document); ok {
							isVoice := m.Voice
							isRound := m.Round
							durationSec := 0
							origFileName := ""
							for _, attr := range doc.Attributes {
								switch a := attr.(type) {
								case *tg.DocumentAttributeAudio:
									if a.Voice {
										isVoice = true
										durationSec = a.Duration
									}
								case *tg.DocumentAttributeVideo:
									if a.RoundMessage {
										isRound = true
										durationSec = int(a.Duration)
									}
								case *tg.DocumentAttributeFilename:
									origFileName = a.FileName
								}
							}

							loc := &tg.InputDocumentFileLocation{
								ID:            doc.ID,
								AccessHash:    doc.AccessHash,
								FileReference: doc.FileReference,
							}

							if isVoice || isRound {
								ext := ".ogg"
								prefix := "voice"
								if isRound || strings.Contains(doc.MimeType, "mp4") {
									ext = ".mp4"
									prefix = "round_voice"
								}
								fileName := fmt.Sprintf("%s_%s_msg%d%s", prefix, msgTime.Format("20060102"), msg.ID, ext)
								filePath := filepath.Join(folderPath, "voice", fileName)
								if _, err := dl.Download(client.API(), loc).ToPath(ctx, filePath); err == nil {
									voiceCount++
									ar := s.ai.AnalyzeMedia(ctx, "voice", filePath, msg.Message, false)
									transcript := ar.Text
									rec.MediaType = "voice"
									rec.MediaFile = fileName
									rec.VoiceTranscript = transcript
									mediaItems = append(mediaItems, storage.MediaItemRecord{
										MessageID:     msg.ID,
										MediaType:     "voice",
										FileName:      fileName,
										FilePath:      filePath,
										Sender:        sender,
										Timestamp:     rec.Timestamp,
										DurationSec:   durationSec,
										FileSizeBytes: doc.Size,
										AIText:        transcript,
										DeepLink:      deepLink,
										Skill:         ar.Skill,
										SkillHash:     ar.SkillHash,
									})
									logf("Saved playable voice -> voice/%s", fileName)
								}
							} else if doc.Size < 25*1024*1024 {
								if origFileName == "" {
									ext := extensionFromMime(doc.MimeType)
									origFileName = fmt.Sprintf("media_%d%s", msg.ID, ext)
								}
								baseNoExt := strings.TrimSuffix(origFileName, filepath.Ext(origFileName))
								ext := filepath.Ext(origFileName)
								if ext == "" {
									ext = extensionFromMime(doc.MimeType)
								}
								fileName := fmt.Sprintf("msg%d_%s%s", msg.ID, config.Slugify(baseNoExt), ext)
								filePath := filepath.Join(folderPath, "documents", fileName)
								if _, err := dl.Download(client.API(), loc).ToPath(ctx, filePath); err == nil {
									docCount++
									ar := s.ai.AnalyzeMedia(ctx, "document", filePath, msg.Message, false)
									aiDesc := ar.Text
									rec.MediaType = "document"
									rec.MediaFile = fileName
									rec.MediaAnalysis = aiDesc
									mediaItems = append(mediaItems, storage.MediaItemRecord{
										MessageID:     msg.ID,
										MediaType:     "document",
										FileName:      fileName,
										FilePath:      filePath,
										Sender:        sender,
										Timestamp:     rec.Timestamp,
										FileSizeBytes: doc.Size,
										Caption:       msg.Message,
										AIText:        aiDesc,
										DeepLink:      deepLink,
										Skill:         ar.Skill,
										SkillHash:     ar.SkillHash,
									})
									logf("Saved document -> documents/%s", fileName)
								}
							}
						}
					}
				}

				messages = append(messages, rec)
			}
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	sort.Slice(messages, func(i, j int) bool {
		return messages[i].MessageID < messages[j].MessageID
	})

	logf("Synthesizing executive summary via %s with skill '%s' (%d messages, %d voice, %d images, %d docs)...",
		s.cfg.ActiveModelLabel(), skillName, len(messages), voiceCount, imgCount, docCount)

	sum := s.ai.GeneratePeriodSummary(
		ctx,
		chat.Title,
		periodLabel,
		skillName,
		notes,
		messages,
		mediaItems,
	)

	record := storage.PeriodSummaryRecord{
		FolderPath:         folderPath,
		ChatID:             chat.ChatID,
		ChatTitle:          chat.Title,
		PeriodSpec:         periodSpec,
		PeriodStart:        startTime.Format("2006-01-02"),
		PeriodEnd:          endTime.Format("2006-01-02"),
		Prompt:             notes,
		Skill:              sum.Skill,
		SkillHash:          sum.SkillHash,
		Status:             "completed",
		ModelUsed:          s.cfg.ActiveModelLabel(),
		TotalMessages:      len(messages),
		VoiceCount:         voiceCount,
		ImageCount:         imgCount,
		DocumentCount:      docCount,
		ExecutiveSummary:   sum.ExecutiveSummary,
		KeyDecisions:       sum.KeyDecisions,
		VoiceMediaInsights: sum.VoiceMediaInsights,
		ActionItems:        sum.ActionItems,
		CreatedAt:          time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		MediaItems:         mediaItems,
		Messages:           messages,
	}

	if err := storage.SavePeriodRun(record); err != nil {
		return nil, err
	}
	logf("Saved period archive -> %s", folderPath)
	return &record, nil
}

func (s *Service) generateOfflineDemoPeriod(
	ctx context.Context,
	chat storage.ChatInfo,
	periodSpec string,
	startTime, endTime time.Time,
	periodLabel, skillName, notes, folderPath string,
) (*storage.PeriodSummaryRecord, error) {
	voiceFile := filepath.Join(folderPath, "voice", "voice_sample_msg101.ogg")
	imgFile := filepath.Join(folderPath, "images", "img_sample_msg104.png")
	docFile := filepath.Join(folderPath, "documents", "msg108_Sprint_Notes.md")

	_ = os.WriteFile(voiceFile, []byte("OGG_VOICE_DATA"), 0o644)
	_ = os.WriteFile(strings.TrimSuffix(voiceFile, ".ogg")+".txt", []byte("Team, let's lock the release scope for this period and verify all MTProto connection tests before Friday."), 0o644)
	_ = os.WriteFile(imgFile, []byte("PNG_IMAGE_DATA"), 0o644)
	_ = os.WriteFile(docFile, []byte("# Sprint Notes\n- Finalized Go + Bubble Tea TUI\n- Verified MTProto DC 2 connection\n"), 0o644)

	msgs := []storage.MessageRecord{
		{
			MessageID:       101,
			Sender:          chat.Title,
			Timestamp:       startTime.Add(24 * time.Hour).Format("2006-01-02 11:00"),
			MediaType:       "voice",
			MediaFile:       filepath.Base(voiceFile),
			VoiceTranscript: "Team, let's lock the release scope for this period and verify all MTProto connection tests before Friday.",
			DeepLink:        "https://t.me/c/100/101",
		},
		{
			MessageID:     104,
			Sender:        chat.Title,
			Timestamp:     startTime.Add(48 * time.Hour).Format("2006-01-02 15:30"),
			Text:          "Architecture overview diagram attached.",
			MediaType:     "image",
			MediaFile:     filepath.Base(imgFile),
			MediaAnalysis: "System architecture diagram showing MTProto client, local media vault, and Gemini summarizer.",
			DeepLink:      "https://t.me/c/100/104",
		},
		{
			MessageID:     108,
			Sender:        chat.Title,
			Timestamp:     endTime.Add(-12 * time.Hour).Format("2006-01-02 17:15"),
			Text:          "Sprint summary notes attached.",
			MediaType:     "document",
			MediaFile:     filepath.Base(docFile),
			MediaAnalysis: "Markdown sprint notes covering Go + Bubble Tea migration and DC 2 verification.",
			DeepLink:      "https://t.me/c/100/108",
		},
	}

	mediaItems := []storage.MediaItemRecord{
		{
			MessageID:     101,
			MediaType:     "voice",
			FileName:      filepath.Base(voiceFile),
			FilePath:      voiceFile,
			Sender:        chat.Title,
			Timestamp:     msgs[0].Timestamp,
			DurationSec:   32,
			FileSizeBytes: 64200,
			AIText:        msgs[0].VoiceTranscript,
			DeepLink:      msgs[0].DeepLink,
		},
		{
			MessageID:     104,
			MediaType:     "image",
			FileName:      filepath.Base(imgFile),
			FilePath:      imgFile,
			Sender:        chat.Title,
			Timestamp:     msgs[1].Timestamp,
			FileSizeBytes: 184000,
			Caption:       msgs[1].Text,
			AIText:        msgs[1].MediaAnalysis,
			DeepLink:      msgs[1].DeepLink,
		},
		{
			MessageID:     108,
			MediaType:     "document",
			FileName:      filepath.Base(docFile),
			FilePath:      docFile,
			Sender:        chat.Title,
			Timestamp:     msgs[2].Timestamp,
			FileSizeBytes: 4200,
			Caption:       msgs[2].Text,
			AIText:        msgs[2].MediaAnalysis,
			DeepLink:      msgs[2].DeepLink,
		},
	}

	sum := s.ai.GeneratePeriodSummary(
		ctx, chat.Title, periodLabel, skillName, notes, msgs, mediaItems,
	)

	rec := storage.PeriodSummaryRecord{
		FolderPath:         folderPath,
		ChatID:             chat.ChatID,
		ChatTitle:          chat.Title,
		PeriodSpec:         periodSpec,
		PeriodStart:        startTime.Format("2006-01-02"),
		PeriodEnd:          endTime.Format("2006-01-02"),
		Prompt:             notes,
		Skill:              sum.Skill,
		SkillHash:          sum.SkillHash,
		Status:             "completed",
		ModelUsed:          s.cfg.ActiveModelLabel(),
		TotalMessages:      len(msgs),
		VoiceCount:         1,
		ImageCount:         1,
		DocumentCount:      1,
		ExecutiveSummary:   sum.ExecutiveSummary,
		KeyDecisions:       sum.KeyDecisions,
		VoiceMediaInsights: sum.VoiceMediaInsights,
		ActionItems:        sum.ActionItems,
		CreatedAt:          time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
		MediaItems:         mediaItems,
		Messages:           msgs,
	}
	_ = storage.SavePeriodRun(rec)
	return &rec, nil
}

func extensionFromMime(mimeType string) string {
	m := strings.ToLower(strings.TrimSpace(mimeType))
	switch {
	case strings.Contains(m, "mp4"):
		return ".mp4"
	case strings.Contains(m, "ogg") || strings.Contains(m, "opus"):
		return ".ogg"
	case strings.Contains(m, "mpeg") || strings.Contains(m, "mp3"):
		return ".mp3"
	case strings.Contains(m, "wav"):
		return ".wav"
	case strings.Contains(m, "pdf"):
		return ".pdf"
	case strings.Contains(m, "webp"):
		return ".webp"
	case strings.Contains(m, "gif"):
		return ".gif"
	case strings.Contains(m, "png"):
		return ".png"
	case strings.Contains(m, "jpeg") || strings.Contains(m, "jpg"):
		return ".jpg"
	default:
		return ".bin"
	}
}


