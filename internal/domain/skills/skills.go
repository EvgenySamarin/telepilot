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

// EnsureDefaults writes the bundled default skills into dir if the directory is fresh
// or has not been initialized yet. Once initialized, it creates a .initialized marker
// so that user deletions are preserved and defaults are not resurrected on every restart.
func EnsureDefaults(dir string) error {
	if dir == "" {
		return nil
	}

	marker := filepath.Join(dir, ".initialized")
	if _, err := os.Stat(marker); err == nil {
		// Already initialized: respect user deletions and edits
		return nil
	}

	// If directory already has existing skills, mark as initialized without resurrecting deleted ones
	if entries, err := os.ReadDir(dir); err == nil {
		hasSkills := false
		for _, e := range entries {
			if !strings.HasPrefix(e.Name(), ".") {
				hasSkills = true
				break
			}
		}
		if hasSkills {
			_ = os.WriteFile(marker, []byte("initialized\n"), 0o644)
			return nil
		}
	}

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	walkErr := fs.WalkDir(defaultsFS, "defaults", func(p string, d fs.DirEntry, err error) error {
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
	if walkErr == nil {
		_ = os.WriteFile(marker, []byte("initialized\n"), 0o644)
	}
	return walkErr
}

// ResetDefaults restores all bundled default skills from defaultsFS into dir,
// ensuring any missing or modified default files are restored,
// while preserving any custom user-created skills.
func ResetDefaults(dir string) error {
	if dir == "" {
		return fmt.Errorf("empty skills directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	marker := filepath.Join(dir, ".initialized")
	walkErr := fs.WalkDir(defaultsFS, "defaults", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel("defaults", p)
		target := filepath.Join(dir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, readErr := defaultsFS.ReadFile(p)
		if readErr != nil {
			return readErr
		}
		return os.WriteFile(target, data, 0o644)
	})
	if walkErr == nil {
		_ = os.WriteFile(marker, []byte("initialized\n"), 0o644)
	}
	return walkErr
}

// Load parses every <dir>/<name>/SKILL.md (and also normalizes standalone .md files
// or single-.md folders added via macOS Finder). Broken skills are skipped and reported in errs.
func Load(dir string) (list []Skill, errs []error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}

	// Track existing directory slugs so we can detect Finder folder duplications
	dirNames := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirNames[Slug(e.Name())] = true
		}
	}

	// First pass: if user dropped a standalone <name>.md file directly into .agents/skills/ via Finder,
	// automatically organize it into .agents/skills/<slug>/SKILL.md.
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		lower := strings.ToLower(e.Name())
		if !strings.HasSuffix(lower, ".md") || lower == "readme.md" {
			continue
		}
		rawName := strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		slug := Slug(rawName)
		if slug == "" {
			continue
		}
		srcFile := filepath.Join(dir, e.Name())
		targetDir := filepath.Join(dir, slug)
		targetFile := filepath.Join(targetDir, FileName)
		if _, statErr := os.Stat(targetFile); statErr != nil {
			if data, readErr := os.ReadFile(srcFile); readErr == nil {
				normalized := EnsureFrontmatter(slug, data)
				if mkErr := os.MkdirAll(targetDir, 0o755); mkErr == nil {
					if wErr := os.WriteFile(targetFile, normalized, 0o644); wErr == nil {
						_ = os.Remove(srcFile)
						dirNames[slug] = true
					}
				}
			}
		}
	}

	// Re-read directory entries in case standalone .md files were promoted into folders
	entries, err = os.ReadDir(dir)
	if err != nil {
		return nil, []error{err}
	}

	seenNames := make(map[string]bool, len(entries))
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		folderPath := filepath.Join(dir, e.Name())
		folderSlug := Slug(e.Name())
		path := filepath.Join(folderPath, FileName)
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			// Check if the user placed a single .md file inside the subfolder via Finder
			if altPath, altData, ok := findSingleMarkdownInDir(folderPath); ok {
				path = altPath
				data = altData
			} else {
				continue // directory without SKILL.md
			}
		}
		normalized := EnsureFrontmatter(folderSlug, data)
		sk, parseErr := Parse(path, normalized)
		if parseErr != nil {
			errs = append(errs, fmt.Errorf("%s: %w", path, parseErr))
			continue
		}
		if sk.Name == "" {
			sk.Name = folderSlug
		}
		// If user duplicated a skill folder in Finder (e.g. "period-summary copy") without
		// editing the frontmatter `name:`, prefer the folder slug so both skills are available.
		if folderSlug != "" && sk.Name != folderSlug && (dirNames[sk.Name] || seenNames[sk.Name]) {
			sk.Name = folderSlug
		}
		seenNames[sk.Name] = true
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

func findSingleMarkdownInDir(folderPath string) (string, []byte, bool) {
	subEntries, err := os.ReadDir(folderPath)
	if err != nil {
		return "", nil, false
	}
	var mdFiles []string
	for _, se := range subEntries {
		if se.IsDir() || strings.HasPrefix(se.Name(), ".") {
			continue
		}
		lower := strings.ToLower(se.Name())
		if strings.HasSuffix(lower, ".md") && lower != "readme.md" {
			mdFiles = append(mdFiles, filepath.Join(folderPath, se.Name()))
		}
	}
	if len(mdFiles) != 1 {
		return "", nil, false
	}
	data, err := os.ReadFile(mdFiles[0])
	if err != nil {
		return "", nil, false
	}
	return mdFiles[0], data, true
}

// EnsureFrontmatter wraps plain Markdown instructions in a default summary skill frontmatter
// if the file does not already start with "---". If it starts with "---", it is returned as-is.
func EnsureFrontmatter(fallbackSlug string, data []byte) []byte {
	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || strings.HasPrefix(content, "---\n") || strings.HasPrefix(trimmed, "---\n") {
		if !strings.HasPrefix(content, "---\n") && strings.HasPrefix(trimmed, "---\n") {
			return []byte(trimmed + "\n")
		}
		return data
	}
	slug := Slug(fallbackSlug)
	if slug == "" {
		slug = "custom-skill"
	}
	desc := "Custom AI skill (" + slug + ")"
	for _, line := range strings.Split(trimmed, "\n") {
		clean := strings.TrimSpace(strings.TrimLeft(line, "#*-• "))
		if clean != "" {
			if len(clean) > 72 {
				clean = clean[:72] + "..."
			}
			desc = strings.ReplaceAll(clean, ":", " -")
			break
		}
	}
	wrapped := fmt.Sprintf("---\nname: %s\ndescription: %s\nkind: summary\n---\n%s\n", slug, desc, trimmed)
	return []byte(wrapped)
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
description: Describe what this skill does for the selected chat
kind: summary
---
Write the instructions for the AI model here.

You can instruct the model to analyze the conversation, extract decisions, audit code,
translate messages, or produce any custom output in Tab 3 (AI Output) and live chat replies.
`, slug)
	if err := os.WriteFile(path, []byte(tmpl), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Import imports one or more skills from srcPath (which may be a SKILL.md file, any .md file,
// a skill directory containing SKILL.md, or a parent directory of skill folders) into destDir.
func Import(destDir, srcPath string) ([]Skill, error) {
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		return nil, fmt.Errorf("empty skill source path")
	}
	st, err := os.Stat(srcPath)
	if err != nil {
		return nil, err
	}

	if !st.IsDir() {
		sk, err := importSingleSkillFile(destDir, srcPath, strings.EqualFold(filepath.Base(srcPath), FileName))
		if err != nil {
			return nil, err
		}
		return []Skill{sk}, nil
	}

	// srcPath is a directory: check if it is itself a skill folder
	skillMd := filepath.Join(srcPath, FileName)
	if info, statErr := os.Stat(skillMd); statErr == nil && !info.IsDir() {
		sk, err := importSingleSkillFile(destDir, skillMd, true)
		if err != nil {
			return nil, err
		}
		return []Skill{sk}, nil
	}
	if altMd, _, ok := findSingleMarkdownInDir(srcPath); ok {
		sk, err := importSingleSkillFile(destDir, altMd, true)
		if err != nil {
			return nil, err
		}
		return []Skill{sk}, nil
	}

	// Otherwise, check if srcPath is a parent directory containing skill subfolders or .md files
	entries, err := os.ReadDir(srcPath)
	if err != nil {
		return nil, err
	}
	var imported []Skill
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		subPath := filepath.Join(srcPath, e.Name())
		if e.IsDir() {
			subSkillMd := filepath.Join(subPath, FileName)
			if info, statErr := os.Stat(subSkillMd); statErr == nil && !info.IsDir() {
				if sk, impErr := importSingleSkillFile(destDir, subSkillMd, true); impErr == nil {
					imported = append(imported, sk)
				}
			} else if altMd, _, ok := findSingleMarkdownInDir(subPath); ok {
				if sk, impErr := importSingleSkillFile(destDir, altMd, true); impErr == nil {
					imported = append(imported, sk)
				}
			}
		} else if strings.HasSuffix(strings.ToLower(e.Name()), ".md") && strings.ToLower(e.Name()) != "readme.md" {
			if sk, impErr := importSingleSkillFile(destDir, subPath, false); impErr == nil {
				imported = append(imported, sk)
			}
		}
	}
	if len(imported) == 0 {
		return nil, fmt.Errorf("no valid SKILL.md or .md skill files found in %s", srcPath)
	}
	return imported, nil
}

func importSingleSkillFile(destDir, mdFilePath string, copySiblingFiles bool) (Skill, error) {
	data, err := os.ReadFile(mdFilePath)
	if err != nil {
		return Skill{}, err
	}

	base := filepath.Base(mdFilePath)
	var fallbackSlug string
	if strings.EqualFold(base, FileName) {
		fallbackSlug = Slug(filepath.Base(filepath.Dir(mdFilePath)))
	} else {
		fallbackSlug = Slug(strings.TrimSuffix(base, filepath.Ext(base)))
	}
	if fallbackSlug == "" || fallbackSlug == "skills" {
		fallbackSlug = "imported-skill"
	}

	normalized := EnsureFrontmatter(fallbackSlug, data)
	parsed, err := Parse(mdFilePath, normalized)
	if err != nil {
		return Skill{}, err
	}

	slug := Slug(parsed.Name)
	if slug == "" {
		slug = fallbackSlug
	}
	if parsed.Name == "" {
		parsed.Name = slug
	}

	targetDir := filepath.Join(destDir, slug)
	targetFile := filepath.Join(targetDir, FileName)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return Skill{}, err
	}

	// Copy sibling files if importing from a dedicated skill folder (and not already in targetDir)
	srcDir := filepath.Dir(mdFilePath)
	if copySiblingFiles && filepath.Clean(srcDir) != filepath.Clean(targetDir) && filepath.Clean(srcDir) != filepath.Clean(destDir) {
		if entries, readErr := os.ReadDir(srcDir); readErr == nil {
			for _, e := range entries {
				if e.IsDir() || strings.HasPrefix(e.Name(), ".") || filepath.Clean(filepath.Join(srcDir, e.Name())) == filepath.Clean(mdFilePath) {
					continue
				}
				if b, rErr := os.ReadFile(filepath.Join(srcDir, e.Name())); rErr == nil {
					_ = os.WriteFile(filepath.Join(targetDir, e.Name()), b, 0o644)
				}
			}
		}
	}

	if err := os.WriteFile(targetFile, normalized, 0o644); err != nil {
		return Skill{}, err
	}
	parsed.Path = targetFile
	return parsed, nil
}
