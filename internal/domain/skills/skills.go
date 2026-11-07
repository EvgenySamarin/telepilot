// Package skills loads AI instruction "skills" from <project>/.agents/skills/<name>/SKILL.md.
//
// A skill is a Markdown file with a small YAML-like frontmatter header:
//
//	---
//	name: period-summary
//	description: Executive summary of a chat period
//	kind: summary            # "summary" or "media"
//	media: voice, audio      # media skills only: which media kinds it handles ("*" = fallback)
//	---
//	<instructions for the model>
//
// The application reads the skill body and inlines it into the agy prompt, so skill
// behaviour is deterministic and independent of agy's own skill discovery.
package skills

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

//go:embed defaults
var defaultsFS embed.FS

const (
	KindSummary = "summary"
	KindMedia   = "media"

	FileName = "SKILL.md"
)

// Media kinds used to route media files to media skills.
const (
	MediaVoice     = "voice"      // Telegram voice notes (.ogg/opus)
	MediaVideoNote = "video_note" // Telegram round video messages
	MediaAudio     = "audio"      // music / audio files sent as documents
	MediaVideo     = "video"      // videos & GIF animations
	MediaImage     = "image"      // photos & image files
	MediaSticker   = "sticker"    // .webp / .tgs stickers
	MediaDocument  = "document"   // everything else (PDF, text, archives, ...)
	MediaAny       = "*"          // fallback marker in frontmatter
)

// AllMediaKinds lists every media kind in display order.
var AllMediaKinds = []string{
	MediaVoice, MediaVideoNote, MediaAudio, MediaVideo, MediaImage, MediaSticker, MediaDocument,
}

type Skill struct {
	Name        string
	Description string
	Kind        string
	Media       []string
	Path        string
	Body        string
	Hash        string // sha256 of the full SKILL.md content (first 12 hex chars)
}

// EnsureDefaults writes the bundled default skills into dir, never overwriting
// files that already exist (so user edits are preserved).
func EnsureDefaults(dir string) error {
	return fs.WalkDir(defaultsFS, "defaults", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("defaults", p)
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if _, statErr := os.Stat(target); statErr == nil {
			return nil
		}
		data, readErr := defaultsFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, data, 0o644)
	})
}

// Load parses every <dir>/<name>/SKILL.md. Broken skills are skipped and reported in errs.
func Load(dir string) (list []Skill, errs []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		path := filepath.Join(dir, e.Name(), FileName)
		data, err := os.ReadFile(path)
		if err != nil {
			continue // directory without SKILL.md
		}
		sk, err := Parse(path, data)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, err))
			continue
		}
		if sk.Name == "" {
			sk.Name = e.Name()
		}
		list = append(list, sk)
	}
	sort.SliceStable(list, func(i, j int) bool {
		if list[i].Kind != list[j].Kind {
			return list[i].Kind == KindSummary
		}
		return list[i].Name < list[j].Name
	})
	return list, errs
}

// Parse parses SKILL.md content.
func Parse(path string, data []byte) (Skill, error) {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	sum := sha256.Sum256([]byte(content))
	sk := Skill{Path: path, Kind: KindSummary, Hash: hex.EncodeToString(sum[:])[:12]}

	if !strings.HasPrefix(content, "---\n") {
		return sk, fmt.Errorf("missing frontmatter (file must start with '---')")
	}
	rest := content[len("---\n"):]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return sk, fmt.Errorf("unterminated frontmatter (missing closing '---')")
	}
	header := rest[:end]
	body := rest[end+len("\n---"):]
	if nl := strings.IndexByte(body, '\n'); nl >= 0 {
		body = body[nl+1:]
	} else {
		body = ""
	}
	sk.Body = strings.TrimSpace(body)

	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, ':')
		if idx <= 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:idx]))
		val := stripInlineComment(strings.TrimSpace(line[idx+1:]))
		val = strings.Trim(val, `"'`)
		switch key {
		case "name":
			sk.Name = val
		case "description":
			sk.Description = val
		case "kind":
			sk.Kind = strings.ToLower(val)
		case "media":
			val = strings.Trim(val, "[]")
			for _, p := range strings.Split(val, ",") {
				p = strings.ToLower(strings.Trim(strings.TrimSpace(p), `"'`))
				if p != "" {
					sk.Media = append(sk.Media, p)
				}
			}
		}
	}

	switch sk.Kind {
	case KindSummary:
	case KindMedia:
		if len(sk.Media) == 0 {
			return sk, fmt.Errorf("media skill must declare 'media:' kinds (e.g. 'media: voice, audio' or 'media: *')")
		}
	default:
		return sk, fmt.Errorf("unknown kind %q (expected 'summary' or 'media')", sk.Kind)
	}
	if sk.Body == "" {
		return sk, fmt.Errorf("skill body is empty")
	}
	return sk, nil
}

func stripInlineComment(v string) string {
	if strings.HasPrefix(v, `"`) || strings.HasPrefix(v, `'`) {
		return v
	}
	if idx := strings.Index(v, " #"); idx >= 0 {
		return strings.TrimSpace(v[:idx])
	}
	return v
}

// Find returns the skill with the given name.
func Find(list []Skill, name string) (Skill, bool) {
	name = strings.TrimSpace(name)
	for _, s := range list {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

// Summaries returns only summary skills.
func Summaries(list []Skill) []Skill {
	var out []Skill
	for _, s := range list {
		if s.Kind == KindSummary {
			out = append(out, s)
		}
	}
	return out
}

// ForMedia returns the media skill handling the given media kind:
// an exact match first, then a "*" fallback skill.
func ForMedia(list []Skill, mediaKind string) (Skill, bool) {
	var fallback *Skill
	for i := range list {
		s := list[i]
		if s.Kind != KindMedia {
			continue
		}
		for _, m := range s.Media {
			if m == mediaKind {
				return s, true
			}
			if m == MediaAny && fallback == nil {
				fallback = &list[i]
			}
		}
	}
	if fallback != nil {
		return *fallback, true
	}
	return Skill{}, false
}

var (
	videoExts   = map[string]bool{".mp4": true, ".mov": true, ".webm": true, ".mkv": true, ".avi": true, ".m4v": true, ".gif": true, ".3gp": true}
	audioExts   = map[string]bool{".mp3": true, ".m4a": true, ".wav": true, ".flac": true, ".ogg": true, ".oga": true, ".opus": true, ".aac": true, ".aiff": true}
	imageExts   = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".heic": true, ".heif": true, ".bmp": true, ".tif": true, ".tiff": true}
	stickerExts = map[string]bool{".webp": true, ".tgs": true}
)

// ClassifyMedia maps a stored media item (app-level media type + file name) to a media kind.
func ClassifyMedia(mediaType, fileName string) string {
	ext := strings.ToLower(filepath.Ext(fileName))
	base := strings.ToLower(filepath.Base(fileName))
	switch mediaType {
	case "voice":
		if strings.HasPrefix(base, "round_voice") || videoExts[ext] {
			return MediaVideoNote
		}
		return MediaVoice
	case "image":
		if stickerExts[ext] {
			return MediaSticker
		}
		return MediaImage
	}
	switch {
	case videoExts[ext]:
		return MediaVideo
	case audioExts[ext]:
		return MediaAudio
	case stickerExts[ext]:
		return MediaSticker
	case imageExts[ext]:
		return MediaImage
	default:
		return MediaDocument
	}
}

var slugRe = regexp.MustCompile(`[^a-z0-9-]+`)

// Slug normalizes a user-entered skill name into a folder-safe identifier.
func Slug(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	s = slugRe.ReplaceAllString(s, "")
	s = strings.Trim(s, "-")
	for strings.Contains(s, "--") {
		s = strings.ReplaceAll(s, "--", "-")
	}
	return s
}

// Create scaffolds a new summary skill from a template and returns its SKILL.md path.
func Create(dir, name string) (string, error) {
	slug := Slug(name)
	if slug == "" {
		return "", fmt.Errorf("invalid skill name %q", name)
	}
	skillDir := filepath.Join(dir, slug)
	path := filepath.Join(skillDir, FileName)
	if _, err := os.Stat(path); err == nil {
		return path, fmt.Errorf("skill %q already exists", slug)
	}
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		return "", err
	}
	tmpl := fmt.Sprintf(`---
name: %s
description: Describe what this skill produces (shown in the Re-Prompt tab)
# kind: "summary" summarizes a whole chat period.
#       "media" analyzes single media files; then also set e.g. "media: voice, audio"
#       (kinds: voice, video_note, audio, video, image, sticker, document, or * for fallback).
kind: summary
---
Write the instructions for the model here.

For summary skills, the app always asks for JSON with these 4 fields, so describe
what each one should contain:
- executive_summary: ...
- key_decisions: ...
- voice_media_insights: ...
- action_items: ...
`, slug)
	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		return "", err
	}
	return path, nil
}
