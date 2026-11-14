# TelePilot (`tgp`)

<p align="center">
  <a href="https://github.com/EvgenySamarin/telepilot/releases/latest"><img src="https://img.shields.io/github/v/release/EvgenySamarin/telepilot?style=flat-square&logo=github" alt="Latest Release" /></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/badge/Language-Go%201.27+-00ADD8?style=flat-square&logo=go" alt="Go" /></a>
  <a href="https://github.com/charmbracelet/bubbletea"><img src="https://img.shields.io/badge/UI-Charm%20Bubble%20Tea-F25D94?style=flat-square" alt="Bubble Tea" /></a>
  <a href="https://www.apple.com/macos/"><img src="https://img.shields.io/badge/Platform-macOS-grey?style=flat-square&logo=apple" alt="macOS" /></a>
  <a href="https://opensource.org/licenses/MIT"><img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" /></a>
</p>

<p align="center">
  A lightweight, keyboard-driven <strong>Telegram AI Client & Skill Runner</strong> for macOS built with <strong>pure Go (<a href="https://github.com/gotd/td">gotd/td</a> MTProto)</strong>, <strong><a href="https://github.com/charmbracelet/bubbletea">Charm Bubble Tea</a> & <a href="https://github.com/charmbracelet/lipgloss">Lipgloss</a></strong>, and <strong>Antigravity CLI (<code>agy</code>)</strong>.
</p>

<p align="center">
  Works <strong>alongside your official Telegram macOS app</strong> (using Telegram's native multi-session capability) to give you a dedicated <strong>Terminal UI</strong> with real-time two-way messaging, per-chat AI auto-reply & single-shot instructions, and flexible AI skill execution.
</p>

---

<p align="center">
  <a href="#-key-features">
    <img src="assets/demo_dashboard.svg" alt="TelePilot Terminal UI — Live Chat & Per-Chat AI" width="100%" />
  </a>
</p>

---

## ⚡ Key Features

### 1. Vim-Style Navigation
- **Keyboard-First Workflow**: Navigate the entire interface seamlessly using standard Vim motions (`h`, `j`, `k`, `l`, `g`, `G`, `Ctrl+d`, `Ctrl+u`).
- **Instant Search**: Filter chats by title or username using `/` or `\`.
- **Card-Based Navigation**: Browse and drill into settings blocks and skill libraries without losing cursor context.

---

### 2. Live Chat & AI Messaging
- **Real-Time Messaging**: Send and receive Telegram messages instantly with live push updates.
- **Chat Selection & Active AI Status**: Select any chat from the sidebar. Chats with active AI assistance are clearly highlighted.
- **Native Media Previews & macOS Finder**: Play audio messages, preview photos and documents via QuickLook, or reveal files in Finder.
- **On-Demand AI Replies & Custom Instructions**: Request an immediate AI response to conversation context, or run targeted single-shot instructions.

---

### 3. Per-Chat AI Settings & Skills Library
Configure AI behavior independently for each Telegram conversation:
- **Autonomous Auto-Reply**: Automatically generate AI responses to incoming messages in chosen chats.
- **Flexible Send Modes**: Choose between drafting messages into the compose box for manual review or sending immediately.
- **Single-Shot Instructions**: Provide ad-hoc, temporary tasks for a chat without changing permanent settings.
- **Background Task Scheduler**: Natural-language reminders and recurring tasks (e.g. "remind me in 30 minutes" or "repeat daily") are automatically parsed and executed by the internal background ticker without requiring rigid command syntax.
- **Modular Skills Library**: Run, author, preview, or import Markdown skills (`SKILL.md`) with automatic hot-reloading. Default skills are seeded on initial launch and preserve user deletions.

<p align="center">
  <img src="assets/skills_browser.svg" alt="TelePilot Per-Chat AI Instructions & 2-Level Box Navigation" width="100%" />
</p>

<p align="center">
  <img src="assets/skill_editor_preview.svg" alt="TelePilot In-TUI Skill Preview and Editor" width="100%" />
</p>

---

### 4. AI Skill Output & Reports
Run specialized skills across conversation history and inspect structured results:
- **One-Action Skill Execution**: Execute the assigned skill on chat history and review the generated report.
- **Detailed Metadata**: Inspect execution timestamps, active model, analyzed message count, and export paths.
- **Exported Reports**: Automatically saved as clean Markdown files that can be opened in QuickLook or revealed in Finder.

<p align="center">
  <img src="assets/skill_output.svg" alt="TelePilot AI Skill Output & Execution Report" width="100%" />
</p>

<p align="center">
  <img src="assets/media_vault.svg" alt="TelePilot Detailed Skill Output & Natural Language Report" width="100%" />
</p>

---

### 5. Global Preferences & Chat Management
- **Localization Support**: Full UI localization across all views and dialogs (currently supports EN and RU).
- **Pinned / Favourite Chats**: Keep your most important conversations pinned to the top of the chat list.
- **Hidden Chats**: Hide noisy or inactive chats to keep your workspace focused.
  - *Sync Optimization*: Hidden chats are excluded from background polling, push updates, and AI auto-replies, reducing network and CPU overhead.
- **Restore Default Skills**: Re-extract bundled default skills from binary memory at any time without affecting custom skills.

<p align="center">
  <img src="assets/language_config.svg" alt="TelePilot Global Configuration & Chat Management" width="100%" />
</p>

---

### 6. Phone QR Login
- Quick and secure authentication by scanning a terminal QR code with your phone.
- Multi-session MTProto connection that runs alongside your official Telegram desktop app.

<p align="center">
  <img src="assets/qr_login.svg" alt="TelePilot MTProto QR Code Login Modal" width="100%" />
</p>

---

## 📁 Data & Skills Architecture

TelePilot stores per-chat configurations and AI skill outputs cleanly in `data/` and modular skills in `.agents/skills/`:

```text
data/
├── chat_ai_config.json      # Per-chat AI status, active skill, auto-reply, send mode & single-shot state
├── chat_skill_outputs.json  # Cached AI skill outputs & execution metadata per chat
├── favorite_chats.json      # Persisted favourite / pinned chats list
├── hidden_chats.json        # Persisted hidden chats list
├── scheduled_tasks.json     # Background AI scheduled reminders & recurring tasks
└── outputs/                 # Exported Markdown output files (<chat_slug>_output.md) for QuickLook & Finder

.agents/skills/              # Modular AI skills (SKILL.md) loaded dynamically with live reload
├── decisions-log/
├── document-summarize/
├── image-describe/
├── media-fallback/
├── period-summary/
├── standup-digest/
├── video-describe/
└── voice-transcribe/
```

### Go Package Layout
```text
cmd/
├── tgp/                     # Primary CLI & TUI entrypoint (telepilot / tgp)
└── screens/                 # Headless SVG screenshot generator for assets/

internal/
├── ai/                      # Antigravity (agy) CLI wrapper, skill runner & schedule parser
├── config/                  # .env & runtime AppConfig loader/persister
├── i18n/                    # EN / RU UI localization
├── mtproto/                 # Telegram gotd/td client, QR auth, live sync & media download
├── skills/                  # Bundled default skills & SKILL.md loader/importer
├── storage/                 # Domain models & JSON/Markdown persistence
└── tui/                     # Bubble Tea Model, views, modals, key handlers & tests
```

See [docs/README.md](docs/README.md) for full generated Go API documentation.

---

## 🚀 Quick Start

### 1. Install `telepilot` (and alias `tgp`)

#### Option A: Download Latest Release (Recommended)
Download the prebuilt archive for your platform (`darwin_arm64` for Apple Silicon, `darwin_amd64` for Intel Mac, or `linux_amd64`) from [GitHub Releases](https://github.com/EvgenySamarin/telepilot/releases/latest):

```bash
# Example for Apple Silicon macOS (replace <TAG> with the release tag, e.g. v0.1.0):
tar -xzf telepilot_<TAG>_darwin_arm64.tar.gz
mkdir -p ~/.local/bin
mv telepilot tgp ~/.local/bin/
chmod +x ~/.local/bin/telepilot ~/.local/bin/tgp
```

*(Ensure `~/.local/bin` is in your `PATH`, e.g. `export PATH="$HOME/.local/bin:$PATH"` in `~/.zshrc`.)*

#### Option B: Build from Source
Run the installer script once to compile from source and place `telepilot` and `tgp` into `~/.local/bin`:

```bash
./install.sh
source ~/.zshrc
```

*(You can also pass a custom alias name: `./install.sh telegent`)*

### 2. Launch the TUI
```bash
telepilot
# or using the 3-letter shortcut:
tgp
```

### 3. Pair with Your Phone
Launch directly into the QR code pairing modal:
```bash
telepilot --qr
```
1. Paste your `TELEGRAM_API_ID` & `TELEGRAM_API_HASH` (from [my.telegram.org/apps](https://my.telegram.org/apps)) and configure your AI engine (Antigravity CLI `agy`).
2. On your phone, open **Telegram → Settings → Devices → Link Desktop Device** and scan the QR code rendered in your terminal.

---

## ⌨️ Keyboard Shortcuts Reference

| Key | Context | Action |
| :--- | :--- | :--- |
| **`1` – `4`** | Global | Switch main tab (`1:Live Chat`, `2:AI & Skills`, `3:AI Output`, `4:Trace`) |
| **`Tab` / `h` / `l`** | Global | Switch focus between Chats sidebar and Main panel |
| **`j` / `k`**, **`g` / `G`** | Global | Vim-style vertical navigation (single line, top, bottom) |
| **`\` / `/`** | Chats Sidebar | Vim-style search through chats by title or username (`Enter`: Open, `Esc`: Clear) |
| **`Enter` / `Space`** | Sidebar | Open and select the highlighted chat `(●)` |
| **`p` / `f`** | Chats Sidebar / Global | Pin or unpin favourite chat on top of the chats list (`📌`) |
| **`H` / `x`** | Chats Sidebar | Hide chat from list (excluded from view; managed in Settings `C`) |
| **`i` / `Enter`** | Tab 1 (`Live Chat`) | Focus the compose box to write and send a message (`Esc` to cancel) |
| **`a`** | Tab 1 (`Live Chat`) | Generate an on-demand AI reply for the active chat |
| **`m`** | Tab 1, 2, 3 | Open **Single-Shot AI Instruction** modal (`Enter`: Replace & Run, `Ctrl+A`: Append, `Ctrl+X`: Stop) |
| **`Enter` / `Esc`** | Tab 2 (`AI & Skills`) | Drill into selected box (`Enter`) / Exit back to box-level navigation (`Esc`) |
| **`a`** | Tab 2 (`AI & Skills`) | Toggle per-chat **AI Auto-Reply** (`[ON]` / `[OFF]`) |
| **`s`** | Tab 2 (`AI & Skills`) | Toggle per-chat **AI Send Mode** (`Draft to Compose Box` vs `Auto-Send Immediately`) |
| **`A`** | Tab 2 (`AI & Skills`) | Trigger an immediate on-demand **AI Reply** for the selected chat |
| **`x`** | Tab 2 (`AI & Skills`) | Disable AI instructions for the selected chat |
| **`v` / `e`** | Tab 2 (`Skills Library`) | Preview skill in-TUI (`v`) or edit `SKILL.md` in `$EDITOR` (`e`) |
| **`O` / `o`** | Tab 2 (`Skills Library`) | Open skill in macOS GUI editor (`O`) or reveal skill in macOS Finder (`o`) |
| **`n` / `I`** | Tab 2 (`Skills Library`) | Create new skill / paste path (`n`) or import skill via native macOS Finder picker (`I`) |
| **`r`** | Tab 1, 2, 3 | Refresh live chat (Tab 1) or run active AI skill on selected chat (Tab 2 & Tab 3) |
| **`v` / `o`** | Tab 1, 2, 3 | Play voice / QuickLook media or reveal in Finder (`o`) (Tab 1), preview / Finder (Tab 2), QuickLook / Finder (`o`) (Tab 3) |
| **`c`** | Global | Open **AI Engine Configuration** modal (Antigravity CLI `agy`) |
| **`C`** | Global | Open **Global Configuration** modal (localization, favourite & hidden chats) |
| **`Ctrl+R`** | Global Config (`C`) | Reset/restore bundled default AI skills from binary memory into `.agents/skills/` |
| **`L`** | Global | Open **MTProto Connection Diagnostics & QR Code Login** modal |
| **`?` / `q`** | Global | Show Help modal (`?`) / Quit (`q`) |

---

## 📄 License

MIT © [Evgeny Samarin](https://github.com/EvgenySamarin)
