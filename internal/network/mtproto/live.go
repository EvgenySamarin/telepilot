/*
File: internal/network/mtproto/live.go
Description: Live Telegram MTProto operations: dialog listing, recent message fetching,
sending messages, media download caching, and real-time update streaming.
*/

package mtproto

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/storage"
)

/*
ListDialogs fetches the user's top chats, groups, and channels.
*/
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

func chatInputPeer(chat storage.ChatInfo) tg.InputPeerClass {
	switch chat.ChatType {
	case "channel", "group":
		if chat.AccessHash != 0 {
			return &tg.InputPeerChannel{
				ChannelID:  chat.ChatID,
				AccessHash: chat.AccessHash,
			}
		}
		return &tg.InputPeerChat{
			ChatID: chat.ChatID,
		}
	default:
		return &tg.InputPeerUser{
			UserID:     chat.ChatID,
			AccessHash: chat.AccessHash,
		}
	}
}

func (s *Service) liveCacheDir(chatID int64) string {
	dir := filepath.Join(s.cfg.DataDir, "live_cache", fmt.Sprintf("%d", chatID))
	_ = os.MkdirAll(dir, 0o755)
	return dir
}

func extractLiveMediaMetadata(msg *tg.Message, msgTime time.Time) (mediaType, fileName string, fileSize int64, durationSec int) {
	if msg.Media == nil {
		return "", "", 0, 0
	}
	switch m := msg.Media.(type) {
	case *tg.MessageMediaPhoto:
		if photo, ok := m.Photo.(*tg.Photo); ok && len(photo.Sizes) > 0 {
			fileName = fmt.Sprintf("img_%s_msg%d.jpg", msgTime.Format("20060102"), msg.ID)
			for _, sz := range photo.Sizes {
				if ps, ok := sz.(*tg.PhotoSize); ok {
					fileSize = int64(ps.Size)
				}
			}
			return "image", fileName, fileSize, 0
		}
	case *tg.MessageMediaDocument:
		if doc, ok := m.Document.(*tg.Document); ok {
			isVoice := m.Voice
			isRound := m.Round
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
			if isVoice || isRound {
				ext := ".ogg"
				prefix := "voice"
				if isRound || strings.Contains(doc.MimeType, "mp4") {
					ext = ".mp4"
					prefix = "round_voice"
				}
				fileName = fmt.Sprintf("%s_%s_msg%d%s", prefix, msgTime.Format("20060102"), msg.ID, ext)
				return "voice", fileName, doc.Size, durationSec
			}
			if origFileName == "" {
				ext := extensionFromMime(doc.MimeType)
				origFileName = fmt.Sprintf("media_%d%s", msg.ID, ext)
			}
			baseNoExt := strings.TrimSuffix(origFileName, filepath.Ext(origFileName))
			ext := filepath.Ext(origFileName)
			if ext == "" {
				ext = extensionFromMime(doc.MimeType)
			}
			fileName = fmt.Sprintf("msg%d_%s%s", msg.ID, config.Slugify(baseNoExt), ext)
			return "document", fileName, doc.Size, 0
		}
	}
	return "", "", 0, 0
}

/*
FetchRecentMessages loads the last `limit` messages (default 50) for a chat in chronological order.
*/
func (s *Service) FetchRecentMessages(ctx context.Context, chat storage.ChatInfo, limit int) ([]storage.LiveMessage, error) {
	if limit <= 0 {
		limit = 50
	}
	authOk, _ := s.CheckAuthorized(ctx)
	if !authOk {
		msgs := storage.DemoLiveMessages(chat)
		cacheDir := s.liveCacheDir(chat.ChatID)
		for i := range msgs {
			if msgs[i].MediaFileName != "" {
				cand := filepath.Join(cacheDir, msgs[i].MediaFileName)
				if _, err := os.Stat(cand); err == nil {
					msgs[i].CachedPath = cand
				}
			}
		}
		return msgs, nil
	}

	cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	var result []storage.LiveMessage
	client := s.newClient(nil)
	cacheDir := s.liveCacheDir(chat.ChatID)

	err := client.Run(cctx, func(ctx context.Context) error {
		histClass, err := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:       chatInputPeer(chat),
			OffsetID:   0,
			OffsetDate: 0,
			AddOffset:  0,
			Limit:      limit,
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

		userMap := make(map[int64]string)
		selfUserIDs := make(map[int64]bool)
		for _, u := range users {
			if usr, ok := u.(*tg.User); ok {
				if usr.Self {
					selfUserIDs[usr.ID] = true
				}
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
			msgTime := time.Unix(int64(msg.Date), 0).Local()
			sender := chat.Title
			isOutgoing := msg.Out
			if msg.Out {
				sender = "You"
			} else if fromUser, ok := msg.FromID.(*tg.PeerUser); ok {
				if selfUserIDs[fromUser.UserID] {
					sender = "You"
					isOutgoing = true
				} else if name, exists := userMap[fromUser.UserID]; exists && name != "" {
					sender = name
				} else {
					sender = fmt.Sprintf("User %d", fromUser.UserID)
				}
			}

			mType, mFile, mSize, mDur := extractLiveMediaMetadata(msg, msgTime)
			cachedPath := ""
			if mFile != "" {
				cand := filepath.Join(cacheDir, mFile)
				if _, err := os.Stat(cand); err == nil {
					cachedPath = cand
				}
			}

			result = append(result, storage.LiveMessage{
				MessageID:     msg.ID,
				ChatID:        chat.ChatID,
				Sender:        sender,
				IsOutgoing:    isOutgoing,
				Timestamp:     msgTime.Format("2006-01-02 15:04"),
				Text:          msg.Message,
				MediaType:     mType,
				MediaFileName: mFile,
				MediaSize:     mSize,
				DurationSec:   mDur,
				CachedPath:    cachedPath,
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].MessageID < result[j].MessageID
	})
	return result, nil
}

/*
SendMessage sends a text message to the specified chat and returns the resulting LiveMessage.
*/
func (s *Service) SendMessage(ctx context.Context, chat storage.ChatInfo, text string) (storage.LiveMessage, error) {
	clean := strings.TrimSpace(text)
	if clean == "" {
		return storage.LiveMessage{}, fmt.Errorf("empty message")
	}

	nowStr := time.Now().Format("15:04")
	outMsg := storage.LiveMessage{
		MessageID:  int(time.Now().Unix() % 100000),
		ChatID:     chat.ChatID,
		Sender:     "You",
		IsOutgoing: true,
		Timestamp:  nowStr,
		Text:       clean,
	}

	authOk, _ := s.CheckAuthorized(ctx)
	if !authOk {
		return outMsg, nil
	}

	cctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()

	client := s.newClient(nil)
	err := client.Run(cctx, func(ctx context.Context) error {
		updsClass, err := client.API().MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:     chatInputPeer(chat),
			Message:  clean,
			RandomID: time.Now().UnixNano(),
		})
		if err != nil {
			return err
		}
		switch u := updsClass.(type) {
		case *tg.UpdateShortSentMessage:
			outMsg.MessageID = u.ID
		case *tg.Updates:
			for _, upd := range u.Updates {
				if nm, ok := upd.(*tg.UpdateNewMessage); ok {
					if m, ok := nm.Message.(*tg.Message); ok {
						outMsg.MessageID = m.ID
					}
				} else if ncm, ok := upd.(*tg.UpdateNewChannelMessage); ok {
					if m, ok := ncm.Message.(*tg.Message); ok {
						outMsg.MessageID = m.ID
					}
				}
			}
		}
		return nil
	})
	return outMsg, err
}

/*
DownloadMessageMedia downloads a message's media attachment on-demand into data/live_cache/<chat_id>/.
*/
func (s *Service) DownloadMessageMedia(ctx context.Context, chat storage.ChatInfo, msg storage.LiveMessage) (string, error) {
	if msg.MediaType == "" {
		return "", fmt.Errorf("message #%d has no media attachment", msg.MessageID)
	}
	if msg.CachedPath != "" {
		if _, err := os.Stat(msg.CachedPath); err == nil {
			return msg.CachedPath, nil
		}
	}

	cacheDir := s.liveCacheDir(chat.ChatID)
	fileName := msg.MediaFileName
	if fileName == "" {
		fileName = fmt.Sprintf("media_msg%d.bin", msg.MessageID)
	}
	targetPath := filepath.Join(cacheDir, fileName)
	if _, err := os.Stat(targetPath); err == nil {
		return targetPath, nil
	}

	// Check if file already exists in any period archive in exports/
	for _, rec := range storage.DiscoverChatPeriodFolders(s.cfg.ExportsDir, chat) {
		for _, it := range rec.MediaItems {
			if (it.MessageID == msg.MessageID || it.FileName == fileName) && it.FilePath != "" {
				if _, err := os.Stat(it.FilePath); err == nil {
					return it.FilePath, nil
				}
			}
		}
	}

	authOk, _ := s.CheckAuthorized(ctx)
	if !authOk {
		_ = os.WriteFile(targetPath, []byte(fmt.Sprintf("OFFLINE_CACHE_%s_%s", strings.ToUpper(msg.MediaType), fileName)), 0o644)
		return targetPath, nil
	}

	cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client := s.newClient(nil)
	dl := downloader.NewDownloader()

	err := client.Run(cctx, func(ctx context.Context) error {
		histClass, err := client.API().MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     chatInputPeer(chat),
			OffsetID: msg.MessageID + 1,
			Limit:    5,
		})
		if err != nil {
			return err
		}

		var rawMsgs []tg.MessageClass
		switch h := histClass.(type) {
		case *tg.MessagesMessages:
			rawMsgs = h.Messages
		case *tg.MessagesMessagesSlice:
			rawMsgs = h.Messages
		case *tg.MessagesChannelMessages:
			rawMsgs = h.Messages
		}

		for _, mc := range rawMsgs {
			tm, ok := mc.(*tg.Message)
			if !ok || tm.ID != msg.MessageID || tm.Media == nil {
				continue
			}
			switch m := tm.Media.(type) {
			case *tg.MessageMediaPhoto:
				if photo, ok := m.Photo.(*tg.Photo); ok && len(photo.Sizes) > 0 {
					thumbType := "x"
					for _, sz := range photo.Sizes {
						if ps, ok := sz.(*tg.PhotoSize); ok {
							thumbType = ps.Type
						}
					}
					loc := &tg.InputPhotoFileLocation{
						ID:            photo.ID,
						AccessHash:    photo.AccessHash,
						FileReference: photo.FileReference,
						ThumbSize:     thumbType,
					}
					_, err := dl.Download(client.API(), loc).ToPath(ctx, targetPath)
					return err
				}
			case *tg.MessageMediaDocument:
				if doc, ok := m.Document.(*tg.Document); ok {
					loc := &tg.InputDocumentFileLocation{
						ID:            doc.ID,
						AccessHash:    doc.AccessHash,
						FileReference: doc.FileReference,
					}
					_, err := dl.Download(client.API(), loc).ToPath(ctx, targetPath)
					return err
				}
			}
		}
		return fmt.Errorf("media for message #%d not found in chat history", msg.MessageID)
	})
	if err != nil {
		return "", err
	}
	return targetPath, nil
}

/*
StartLiveStream runs a persistent gotd/td UpdateDispatcher to deliver incoming messages in real time.
*/
func (s *Service) StartLiveStream(ctx context.Context, onNewMessage func(chatID int64, msg storage.LiveMessage)) {
	if !s.cfg.HasTelegramCredentials() {
		return
	}
	if _, err := os.Stat(s.cfg.SessionFilePath()); err != nil {
		return
	}

	dispatcher := tg.NewUpdateDispatcher()
	handleTGMessage := func(e tg.Entities, m *tg.Message) {
		if m == nil || onNewMessage == nil {
			return
		}
		var peerChatID int64
		sender := "Telegram"
		switch p := m.PeerID.(type) {
		case *tg.PeerChannel:
			peerChatID = p.ChannelID
			if ch, ok := e.Channels[p.ChannelID]; ok && ch != nil {
				sender = ch.Title
			}
		case *tg.PeerChat:
			peerChatID = p.ChatID
			if ch, ok := e.Chats[p.ChatID]; ok && ch != nil {
				sender = ch.Title
			}
		case *tg.PeerUser:
			peerChatID = p.UserID
			if u, ok := e.Users[p.UserID]; ok && u != nil {
				sender = strings.TrimSpace(u.FirstName + " " + u.LastName)
			}
		}
		isOutgoing := m.Out
		if m.Out {
			sender = "You"
		} else if fromUser, ok := m.FromID.(*tg.PeerUser); ok {
			if u, exists := e.Users[fromUser.UserID]; exists && u != nil {
				if u.Self {
					sender = "You"
					isOutgoing = true
				} else {
					name := strings.TrimSpace(u.FirstName + " " + u.LastName)
					if u.Username != "" {
						name = fmt.Sprintf("%s (@%s)", name, u.Username)
					}
					if name != "" {
						sender = name
					}
				}
			}
		}

		msgTime := time.Unix(int64(m.Date), 0).Local()
		mType, mFile, mSize, mDur := extractLiveMediaMetadata(m, msgTime)
		onNewMessage(peerChatID, storage.LiveMessage{
			MessageID:     m.ID,
			ChatID:        peerChatID,
			Sender:        sender,
			IsOutgoing:    isOutgoing,
			Timestamp:     msgTime.Format("2006-01-02 15:04"),
			Text:          m.Message,
			MediaType:     mType,
			MediaFileName: mFile,
			MediaSize:     mSize,
			DurationSec:   mDur,
		})
	}

	dispatcher.OnNewMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewMessage) error {
		if msg, ok := update.Message.(*tg.Message); ok {
			handleTGMessage(e, msg)
		}
		return nil
	})
	dispatcher.OnNewChannelMessage(func(ctx context.Context, e tg.Entities, update *tg.UpdateNewChannelMessage) error {
		if msg, ok := update.Message.(*tg.Message); ok {
			handleTGMessage(e, msg)
		}
		return nil
	})

	client := s.newClient(dispatcher)
	_ = client.Run(ctx, func(ctx context.Context) error {
		st, err := client.Auth().Status(ctx)
		if err != nil || !st.Authorized {
			return nil
		}
		<-ctx.Done()
		return ctx.Err()
	})
}
