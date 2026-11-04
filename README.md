# Telegram AI Client (macOS TUI + Saved Messages Daemon)

A lightweight, keyboard-driven **Telegram AI Summarizer Client** for macOS built with **Telethon (MTProto)**, **Textual (TUI)**, and **Google Gemini / OpenAI Multimodal AI**.

Works **alongside your official Telegram macOS app** (using Telegram's multi-session capability) so you get a dedicated **Tokyo-Night Terminal UI** AND can trigger summaries directly from your **Telegram Saved Messages**.

---

## Key Features

- **Tokyo-Night Terminal UI (TUI)**:
  - **Top Status Bar**: Shows live pipeline progress (`◉ Ready`, `⟳ Transcribing voice...`), active AI model, batch counter (`1/3`), and elapsed timer (`⏱ 12s`).
  - **Multi-Select Chats Sidebar**: Toggle multiple chats/groups/channels with `Space` and press `s` to batch-parse them.
  - **Period Summary Folders Browser**: Browse all created `exports/<chat>/<YYYY-MM-DD_to_YYYY-MM-DD>/` archives on disk.
  - **5 Switchable Main Views (`1`–`5`)**:
    1. `[1:Summary]`: Interior cards for Metadata, Skill Used, Executive Summary, Key Decisions, Voice/Media Accounting, and Action Items Checklist.
    2. `[2:Media Vault]`: Inspects every file saved in `voice/`, `images/`, and `documents/` with its AI description/transcription. Press `v` for **macOS QuickLook (`qlmanage -p`)** or audio playback (`afplay`), and `o` to reveal in **macOS Finder**.
    3. `[3:Transcript & Voice]`: Chronological timeline with voice messages expanded inline into text (`🎙️ Voice Transcript`).
    4. `[4:Skills & Re-Run]`: Modular skill system (`.agents/skills/`) for chat period summarization and media analysis. Select active skills, edit them in `$EDITOR`, create new custom skills, and re-run AI with optional notes.
    5. `[5:Trace: full]`: Real-time execution logs for MTProto calls, media downloads, and AI synthesis.

- **Period-Based Media & Voice Organization**:
  - Defaults to **1 Month (`1m`)** (configurable to `7d`, `2w`, `3m`, or `YYYY-MM-DD..YYYY-MM-DD`).
  - Automatically downloads and organizes attachments per period:
    ```text
    exports/<chat_slug>/<YYYY-MM-DD>_to_<YYYY-MM-DD>/
    ├── images/        # Downloaded photos & diagrams
    ├── documents/     # Downloaded PDFs, CSVs, Office docs
    ├── voice/         # .ogg voice messages + .txt transcriptions
    ├── transcript.md  # Full chronological timeline with inline voice transcripts
    ├── summary.md     # Markdown report
    └── metadata.json  # Structured cache for instant re-prompting
    ```

---

## Quick Start

### 1. Install `telepilot` (and short alias `tgp`) into Your Terminal
Run the installer script once to add `telepilot` and `tgp` to `~/.local/bin` and your `~/.zshrc`:
```bash
./install.sh
source ~/.zshrc
```
*(Or pass your own preferred name: `./install.sh telegent`)*

### 2. Launch the TUI from Any Directory
```bash
telepilot
# or short 3-letter command:
tgp
```

### 3. Authorize with Your Phone via TUI QR Code
Press **`l`** at any time inside the TUI (or launch directly into the QR login dialog):
```bash
telepilot --qr
```
1. If you haven't set `TELEGRAM_API_ID` & `TELEGRAM_API_HASH` (from [https://my.telegram.org/apps](https://my.telegram.org/apps)) yet, paste them right into the TUI modal (along with your `GEMINI_API_KEY` from [https://aistudio.google.com/apikey](https://aistudio.google.com/apikey)).
2. A live **QR Code** renders directly inside the TUI modal (and you can also press `v` to pop open a high-res PNG QR code in macOS QuickLook).
3. On your phone, open **Telegram → Settings → Devices → Link Desktop Device** and scan the QR code.
4. *(If you have 2FA Cloud Password enabled, the modal automatically shows a password box to complete sign-in).*

### 3. Keyboard Shortcuts in TUI
| Key | Action |
| :--- | :--- |
| `L` | Open **MTProto Connection Diagnostic & QR Code Login** modal |
| `Space` | Toggle `[✓]` multi-selection on chats in the left sidebar |
| `a` | Select / Deselect all chats (when focus on chats), or edit run notes |
| `s` / `Enter` | Start parsing & summarizing selected chats (or activate skill in Tab 4) |
| `p` | Change parsing period (Default: `1m` = 1 month) |
| `e` | Open selected skill's `SKILL.md` in `$EDITOR` (e.g. vim, nano, VS Code) |
| `n` | Create a new custom skill in `.agents/skills/` from template |
| `a` / `m` | Edit optional extra notes / instructions for the summary skill |
| `r` | Re-run AI analysis on cached period folder using active skill |
| `c` | Open AI Engine configuration modal (set binary & cycle models with `F2`) |
| `C` | Open **Global Configuration** modal (switch language between En and Ru) |
| `1` – `5` | Switch main view (`Summary`, `Media Vault`, `Transcript`, `Skills & Re-Run`, `Trace`) |
| `o` | Open active period folder in **macOS Finder** |
| `v` | Preview media in **QuickLook** / play audio, or preview skill in embedded TUI window |
| `d` / `x` | Delete selected period folder |
| `?` | Show help modal |
| `q` | Quit |

