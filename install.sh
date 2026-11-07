#!/usr/bin/env zsh
# ============================================================================
# Installer for TelePilot (Personal Telegram AI Client & Co-Pilot — Go + Bubble Tea)
# Usage:
#   ./install.sh              # Builds Go binary & installs `telepilot` + `tgp`
#   ./install.sh <custom>     # Also installs under your custom command name
# ============================================================================

set -e

PROJECT_DIR="${0:A:h}"
BIN_DIR="$HOME/.local/bin"
PRIMARY_NAME="${1:-telepilot}"
SHORT_ALIAS="tgp"

echo "🚀 Building & installing TelePilot (Go + Bubble Tea + gotd/td) from: $PROJECT_DIR"

mkdir -p "$PROJECT_DIR/bin"
mkdir -p "$BIN_DIR"

cd "$PROJECT_DIR"
go build -o "$PROJECT_DIR/bin/tgp" .
cp "$PROJECT_DIR/bin/tgp" "$PROJECT_DIR/bin/telepilot"

create_launcher() {
  local cmd_name="$1"
  local target_path="$BIN_DIR/$cmd_name"
  cat > "$target_path" <<EOF
#!/usr/bin/env zsh
export TELEPILOT_HOME="$PROJECT_DIR"
exec "$PROJECT_DIR/bin/tgp" "\$@"
EOF
  chmod +x "$target_path"
  echo "  ✓ Installed command: $cmd_name -> $target_path"
}

create_launcher "$PRIMARY_NAME"
if [[ "$PRIMARY_NAME" != "$SHORT_ALIAS" ]]; then
  create_launcher "$SHORT_ALIAS"
fi

ZSHRC="$HOME/.zshrc"
touch "$ZSHRC"
if ! grep -q '\.local/bin' "$ZSHRC" 2>/dev/null; then
  echo '' >> "$ZSHRC"
  echo '# TelePilot (Telegram AI Client) CLI path' >> "$ZSHRC"
  echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$ZSHRC"
  echo "  ✓ Added ~/.local/bin to PATH in ~/.zshrc"
fi

export PATH="$BIN_DIR:$PATH"

if [ -d "$PROJECT_DIR/.git" ] && [ -d "$PROJECT_DIR/.githooks" ]; then
  git -C "$PROJECT_DIR" config core.hooksPath .githooks
  chmod +x "$PROJECT_DIR/.githooks/pre-commit"
  echo "  ✓ Configured git pre-commit security hooks (.githooks)"
fi

echo ""
echo "✅ Native Go binary compiled & installed!"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "You can now launch your client from anywhere in your terminal using:"
echo "  • $SHORT_ALIAS                (Fast 3-letter shortcut — Full Bubble Tea TUI)"
echo "  • $PRIMARY_NAME          (Full Bubble Tea TUI)"
echo "  • $SHORT_ALIAS --test         (Run 3-step MTProto connection check in CLI)"
echo "  • $SHORT_ALIAS --qr           (Run Phone QR Code Login directly in CLI)"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
