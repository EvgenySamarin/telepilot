# TelePilot (`tgp`)

<p align="center">
  <img src="https://img.shields.io/badge/Language-Go%201.24+-00ADD8?style=flat-square&logo=go" alt="Go" />
  <img src="https://img.shields.io/badge/UI-Charm%20Bubble%20Tea-F25D94?style=flat-square" alt="Bubble Tea" />
  <img src="https://img.shields.io/badge/Theme-Tokyo--Night-7aa2f7?style=flat-square" alt="Tokyo Night" />
  <img src="https://img.shields.io/badge/Platform-macOS-grey?style=flat-square&logo=apple" alt="macOS" />
  <img src="https://img.shields.io/badge/License-MIT-blue?style=flat-square" alt="License" />
</p>

<p align="center">
  A lightweight, keyboard-driven <strong>Telegram AI Client & Summarizer</strong> for macOS built with <strong>pure Go (<a href="https://github.com/gotd/td">gotd/td</a> MTProto)</strong>, <strong><a href="https://github.com/charmbracelet/bubbletea">Charm Bubble Tea</a> & <a href="https://github.com/charmbracelet/lipgloss">Lipgloss</a></strong>, and <strong>Antigravity CLI (<code>agy</code>) / Google Gemini Multimodal AI</strong>.
</p>

<p align="center">
  Works <strong>alongside your official Telegram macOS app</strong> (using Telegram's native multi-session capability) to give you a dedicated <strong>Tokyo-Night Terminal UI</strong> with real-time two-way messaging, per-chat AI auto-reply & single-shot instructions, multimodal voice/media analysis, and period archives.
</p>

---

<p align="center">
  <a href="#-key-features">
    <img src="assets/demo_dashboard.svg" alt="TelePilot Tokyo-Night Terminal UI — Live Chat & Per-Chat AI" width="100%" />
  </a>
</p>

---

## ⚡ Key Features

### 1. Real-Time Live Chat & AI Message Control (`[1:Live Chat]`)
- **Human-Readable Chat Stream**: Clean chronological message timeline with date dividers (`─── YYYY-MM-DD ───`), grouped consecutive senders, aligned `HH:MM │` columns, and inline voice note transcripts (`🎙️`).
- **Single-Chat Focus & `[AI]` Badge**: Select any dialog in the Chats sidebar with `Enter` or `Space`. Chats with AI enabled are highlighted in purple with an **`[AI]`** badge.
- **Two-Way Messaging & Real-Time Sync**: Press **`i`** or **`Enter`** in Tab 1 to compose and send messages directly over MTProto, backed by live push updates and automatic background sync.
- **AI Reply & Single-Shot Control**:
  - Press **`a`** in Tab 1 (or **`A`** in Tab 2) to generate an on-demand AI reply to the latest conversation context.
  - Press **`m`** in Tab 1 or Tab 2 to launch a standalone **Single-Shot AI Instruction** for the active chat.

---

### 2. Per-Chat AI Instructions, Single-Shot Runner & Skills (`[2:AI & Skills]`)
Configure AI behavior independently for each Telegram chat (persisted in `data/chat_ai_config.json`):
- **Two-Level Box Navigation**:
  - **Level 1 (Box Selection)**: Use **`j` / `k`** to move between the 5 cards (`AI Actions & Shortcuts`, `Per-Chat AI Instructions`, `Common AI Engine Configuration`, `Skills Library`, and `Selected Chat Latest Archive & Run Settings`) without losing cursor focus.
  - **Level 2 (Inside a Box)**: Press **`Enter`** to drill into the highlighted box (border turns green `◆`) to toggle per-chat AI settings or browse and assign skills; press **`Esc`** (or `Backspace`) to step back out to box selection.
- **Per-Chat AI Message Control**:
  - **Colorized Status Badges**: Instant visual clarity with green **`[ON]` / `[ACTIVE]` / `[CONNECTED]`** and red **`[OFF]` / `[NOT CONFIGURED]`** badges across structured indented option lists.
  - **Auto-Reply (`a`)**: Automatically respond to new incoming messages in the selected chat using AI.
  - **Send Mode (`s`)**: Toggle between **`Draft to Compose Box (Enter to confirm)`** (safe review before sending) and **`Auto-Send Immediately`** (hands-free autonomous replies).
- **Standalone Single-Shot AI Instructions (`m`)**:
  - Give the AI ad-hoc instructions for the current chat (e.g., *"send 3 different messages starting with AI prefix every 15 seconds"* or *"reply politely that I'm in a meeting"*), completely separate from period summarization.
  - **`Enter`** cancels any previously running instruction for that chat and starts the new single-shot immediately; **`Ctrl+A`** appends/queues onto the currently running instruction; **`Ctrl+X`** stops and clears the active runner.
- **Modular Skills Library (`.agents/skills/`)**:
  - Pre-installed skills (`period-summary`, `decisions-log`, `standup-digest`, `voice-transcribe`, `document-summarize`, `image-describe`, `video-describe`).
  - Press **`v`** to preview a skill's `SKILL.md` in-TUI, **`e`** to edit it in `$EDITOR`, or **`n`** to scaffold a new skill template.

<p align="center">
  <img src="assets/skills_browser.svg" alt="TelePilot Per-Chat AI Instructions & 2-Level Box Navigation" width="100%" />
</p>

<p align="center">
  <img src="assets/skill_editor_preview.svg" alt="TelePilot In-TUI Skill Preview and Editor" width="100%" />
</p>

---

### 3. Unified Period Archive: Summary, Media Vault & Transcript (`[3:Period Archive]`)
All historical period exports (`exports/<chat>/<YYYY-MM-DD_to_YYYY-MM-DD>/`) are consolidated into a single scrollable view in Tab 3:
- **Executive Summary Cards**: Structured cards for **Metadata**, **Active Skill**, **Executive Summary**, **Key Decisions & Highlights**, **Voice/Media Accounting**, and **Action Items**.
- **Multimodal Media Vault**: Inspect downloaded voice notes, photos, diagrams, and PDFs with automated AI transcription and vision descriptions. Press **`v`** for native macOS QuickLook (`qlmanage -p` / `afplay`) or **`o`** to reveal the export folder in macOS Finder.
- **Chronological Transcript**: Grouped message history with inline voice transcriptions (`🎙️ Voice Transcript`) and deep links.
- **Period Window Selector (`p`)**: Press **`p`** in Tab 3 to switch the summarization time window (`1d`, `3d`, `7d`, `2w`, `1m`, `3m`, or custom `YYYY-MM-DD..YYYY-MM-DD`).

<p align="center">
  <img src="assets/transcript_view.svg" alt="TelePilot Unified Period Archive — Executive Summary & Transcript" width="100%" />
</p>

<p align="center">
  <img src="assets/media_vault.svg" alt="TelePilot Multimodal Media Vault & Timeline in Tab 3" width="100%" />
</p>

---

### 4. Global Configuration & Bilingual Language Switcher (`C`)
TelePilot features first-class internationalization with instant switching between English and Russian:
- Press **`C`** (capital C) anytime to open the **Global Configuration** modal.
- Toggle between **English (`En`)** and **Russian (`Ru — Русский`)** using `Space` or `1`/`2` keys.
- Live localization across all top status bars, sidebars, tabs, cards, modals, and keyboard hints.

<p align="center">
  <img src="assets/language_config.svg" alt="TelePilot Global Configuration & Language Switcher" width="100%" />
</p>

---

### 5. Instant Phone QR Pairing & MTProto Diagnostics (`L` or `--qr`)
Sign in with your phone camera in seconds:
- Press **`L`** anytime in the TUI (or launch with `telepilot --qr`).
- Live rendered Unicode QR code block right in your terminal with automatic MTProto DC ping and connection diagnostics.
- Works alongside your existing Telegram macOS desktop app without terminating existing sessions.

<p align="center">
  <img src="assets/qr_login.svg" alt="TelePilot MTProto QR Code Login Modal" width="100%" />
</p>

---

## 📁 Period-Based Export & Per-Chat Config Architecture

TelePilot stores per-chat AI configurations in `data/chat_ai_config.json` and organizes period summaries cleanly on disk:

```text
data/
└── chat_ai_config.json  # Per-chat AI status, active skill, auto-reply, send mode & single-shot state

exports/<chat_slug>/<YYYY-MM-DD>_to_<YYYY-MM-DD>/
├── images/              # Downloaded photos, diagrams & whiteboard screenshots
├── documents/           # Downloaded PDFs, CSV benchmarks, technical specs
├── voice/               # Downloaded .ogg voice messages + .txt transcripts
├── transcript.md        # Chronological timeline with inline voice transcriptions
├── summary.md           # Markdown executive report
└── metadata.json        # Structured JSON cache for instant re-prompting
```

---

## 🚀 Quick Start

### 1. Install `telepilot` (and alias `tgp`)
Run the installer script once to compile and place `telepilot` and `tgp` into `~/.local/bin`:

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
1. Paste your `TELEGRAM_API_ID` & `TELEGRAM_API_HASH` (from [my.telegram.org/apps](https://my.telegram.org/apps)) and configure your AI engine (`agy` CLI or `GEMINI_API_KEY`).
2. On your phone, open **Telegram → Settings → Devices → Link Desktop Device** and scan the QR code rendered in your terminal.

---

## ⌨️ Keyboard Shortcuts Reference

| Key | Context | Action |
| :--- | :--- | :--- |
| **`1` – `4`** | Global | Switch main tab (`1:Live Chat`, `2:AI & Skills`, `3:Period Archive`, `4:Trace`) |
| **`Tab` / `h` / `l`** | Global | Switch focus between Chats sidebar and Main panel |
| **`j` / `k`**, **`g` / `G`** | Global | Vim-style vertical navigation (single line, top, bottom) |
| **`Enter` / `Space`** | Sidebar | Open and select the highlighted chat `(●)` |
| **`i` / `Enter`** | Tab 1 (`Live Chat`) | Focus the compose box to write and send a message (`Esc` to cancel) |
| **`a`** | Tab 1 (`Live Chat`) | Generate an on-demand AI reply for the active chat |
| **`m`** | Tab 1 & Tab 2 | Open **Single-Shot AI Instruction** modal (`Enter`: Replace & Run, `Ctrl+A`: Append, `Ctrl+X`: Stop) |
| **`Enter` / `Esc`** | Tab 2 (`AI & Skills`) | Drill into selected box (`Enter`) / Exit back to box-level navigation (`Esc`) |
| **`a`** | Tab 2 (`AI & Skills`) | Toggle per-chat **AI Auto-Reply** (`[ON]` / `[OFF]`) |
| **`s`** | Tab 2 (`AI & Skills`) | Toggle per-chat **AI Send Mode** (`Draft to Compose Box` vs `Auto-Send Immediately`) |
| **`A`** | Tab 2 (`AI & Skills`) | Trigger an immediate on-demand **AI Reply** for the selected chat |
| **`x`** | Tab 2 (`AI & Skills`) | Disable AI for the selected chat |
| **`v` / `e` / `n`** | Tab 2 (`Skills Library`) | Preview skill in-TUI (`v`), edit `SKILL.md` in `$EDITOR` (`e`), or create new skill (`n`) |
| **`r`** | Tab 1, 2, 3 | Refresh live chat (Tab 1) or run/re-run AI period summarization (Tab 2 & Tab 3) |
| **`p`** | Tab 3 (`Period Archive`) | Change summarization period window (`7d`, `2w`, `1m`, `3m`, or custom range) |
| **`v` / `o`** | Tab 1 & Tab 3 | QuickLook preview (`v`) or reveal period archive folder in macOS Finder (`o`) |
| **`c`** | Global | Open **AI Engine Configuration** modal (Antigravity CLI `agy` or Gemini API) |
| **`C`** | Global | Open **Global Configuration** modal (switch language between `En` and `Ru`) |
| **`L`** | Global | Open **MTProto Connection Diagnostics & QR Code Login** modal |
| **`?` / `q`** | Global | Show Help modal (`?`) / Quit (`q`) |

---

## 🛠️ Regenerating Assets & Screenshots

TelePilot includes a pure-Go SVG terminal capture utility that renders the Lipgloss Bubble Tea screens directly into vector SVGs with Tokyo-Night styling and macOS window chrome:

```bash
go run ./cmd/screens
```

Outputs are written to `assets/`.

---

## 📄 License

MIT © [Evgeny Samarin](https://github.com/EvgenySamarin)
