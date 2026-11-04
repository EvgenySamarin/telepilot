package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/ai"
	"telepilot/internal/config"
	"telepilot/internal/mtproto"
	"telepilot/internal/skills"
	"telepilot/internal/storage"
	"telepilot/internal/tui"
)

func main() {
	qrFlag := flag.Bool("qr", false, "Log into Telegram by scanning a QR code with your phone")
	authFlag := flag.Bool("auth", false, "Alias for --qr (Phone QR code login)")
	testFlag := flag.Bool("test", false, "Run 3-step MTProto connection diagnostic from CLI")
	demoFlag := flag.Bool("demo", false, "Launch TUI in demo mode")
	sumChat := flag.String("sum", "", "Run one-shot CLI period summary for a chat title")
	periodFlag := flag.String("period", "", "Period to parse (default 1m = 1 month, e.g. 7d, 2w, 1m)")
	skillFlag := flag.String("skill", "", "Summary skill name from .agents/skills (default: DEFAULT_SKILL or period-summary)")
	notesFlag := flag.String("notes", "", "Extra instructions appended to the summary skill for this run")
	promptFlag := flag.String("prompt", "", "Deprecated alias for --notes")
	flag.Parse()

	cfg := config.Load()
	if *periodFlag != "" {
		cfg.DefaultPeriod = *periodFlag
	}
	if *skillFlag != "" {
		cfg.DefaultSkill = *skillFlag
	}
	if *notesFlag != "" {
		cfg.DefaultNotes = *notesFlag
	} else if *promptFlag != "" {
		cfg.DefaultNotes = *promptFlag
	}

	// Seed the bundled default skills into .agents/skills (existing files are never overwritten).
	if err := skills.EnsureDefaults(cfg.SkillsDir()); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not install default skills into %s: %v\n", cfg.SkillsDir(), err)
	}

	aiEngine := ai.NewEngine(cfg)
	mtSvc := mtproto.NewService(cfg, aiEngine)

	if *testFlag {
		runCLIConnectionTest(cfg, mtSvc)
		return
	}

	if *qrFlag || *authFlag {
		runCLIQRLogin(cfg, mtSvc)
		return
	}

	if strings.TrimSpace(*sumChat) != "" {
		runCLISummary(cfg, mtSvc, *sumChat)
		return
	}

	model := tui.NewModel(cfg, aiEngine, mtSvc, *demoFlag)
	p := tea.NewProgram(model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "TelePilot TUI error: %v\n", err)
		os.Exit(1)
	}
}

func runCLIConnectionTest(cfg *config.AppConfig, mtSvc *mtproto.Service) {
	fmt.Printf("TelePilot (Go + gotd/td) MTProto 2.0 Connection Diagnostic\n")
	fmt.Printf("API ID: %d  |  Server: %s:%d (DC %d)\n\n",
		cfg.TelegramAPIID, cfg.TelegramServerIP, cfg.TelegramServerPort, cfg.TelegramDCID)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	var printed int
	ok, lines := mtSvc.TestConnection(ctx, func(updated []string) {
		for printed < len(updated) {
			fmt.Println("  " + updated[printed])
			printed++
		}
	})
	for printed < len(lines) {
		fmt.Println("  " + lines[printed])
		printed++
	}
	fmt.Println()
	if ok {
		fmt.Println("Result: PASSED — Ready for Phone QR Code Login (`tgp` -> press `l` -> press `g`, or `tgp --qr`).")
	} else {
		fmt.Println("Result: FAILED")
		os.Exit(1)
	}
}

func runCLIQRLogin(cfg *config.AppConfig, mtSvc *mtproto.Service) {
	fmt.Printf("TelePilot — Telegram Phone QR Code Login (%s:%d, DC %d)\n\n",
		cfg.TelegramServerIP, cfg.TelegramServerPort, cfg.TelegramDCID)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	user, err := mtSvc.RunQRLogin(
		ctx,
		func(qrArt, loginURL, pngPath string) {
			fmt.Print("\033[2J\033[H")
			fmt.Printf("Scan this QR code in Telegram on your phone:\n")
			fmt.Printf("Telegram -> Settings -> Devices -> Link Desktop Device\n\n")
			fmt.Println(qrArt)
			fmt.Printf("\nHigh-res PNG saved to: %s\n", pngPath)
		},
		func(status string) {
			fmt.Printf("Status: %s\n", status)
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nQR Login failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("\nSuccessfully logged in as %s!\nSession saved to: %s\n", user, cfg.SessionFilePath())
}

func runCLISummary(cfg *config.AppConfig, mtSvc *mtproto.Service, target string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	chat := storage.ChatInfo{
		ChatID:   -1001849201,
		Title:    target,
		ChatType: "group",
	}
	if dialogs, err := mtSvc.ListDialogs(ctx, 50); err == nil {
		for _, d := range dialogs {
			if strings.Contains(strings.ToLower(d.Title), strings.ToLower(target)) {
				chat = d
				break
			}
		}
	}

	rec, err := mtSvc.ParseChatPeriod(ctx, chat, cfg.DefaultPeriod, cfg.DefaultSkill, cfg.DefaultNotes, func(msg string) {
		fmt.Println("• " + msg)
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("\n" + storage.BuildSummaryMarkdown(*rec))
}

