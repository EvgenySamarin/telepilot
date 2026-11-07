package mtproto

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	qrcode "github.com/skip2/go-qrcode"
)

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
