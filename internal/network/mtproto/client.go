package mtproto

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"

	"telepilot/internal/domain/ai"
	"telepilot/internal/domain/config"
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
