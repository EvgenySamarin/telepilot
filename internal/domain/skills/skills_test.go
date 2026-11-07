package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureDefaultsAndLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skills-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	if err := EnsureDefaults(tmpDir); err != nil {
		t.Fatalf("EnsureDefaults failed: %v", err)
	}

	list, errs := Load(tmpDir)
	if len(errs) > 0 {
		t.Fatalf("Load reported errors: %v", errs)
	}

	if len(list) < 5 {
		t.Fatalf("expected at least 5 default skills, got %d", len(list))
	}

	// Verify period-summary exists and is a summary skill
	ps, ok := Find(list, "period-summary")
	if !ok {
		t.Fatalf("period-summary skill not found")
	}
	if ps.Kind != KindSummary {
		t.Fatalf("expected kind %q, got %q", KindSummary, ps.Kind)
	}
	if ps.Hash == "" {
		t.Fatalf("expected non-empty hash")
	}

	// Verify media skill routing
	vSkill, ok := ForMedia(list, MediaVoice)
	if !ok || vSkill.Name != "voice-transcribe" {
		t.Fatalf("expected voice-transcribe for voice, got %v (ok=%v)", vSkill, ok)
	}

	imgSkill, ok := ForMedia(list, MediaImage)
	if !ok || imgSkill.Name != "image-describe" {
		t.Fatalf("expected image-describe for image, got %v (ok=%v)", imgSkill, ok)
	}

	docSkill, ok := ForMedia(list, MediaDocument)
	if !ok || docSkill.Name != "document-summarize" {
		t.Fatalf("expected document-summarize for document, got %v (ok=%v)", docSkill, ok)
	}

	vidSkill, ok := ForMedia(list, MediaVideo)
	if !ok || vidSkill.Name != "video-describe" {
		t.Fatalf("expected video-describe for video, got %v (ok=%v)", vidSkill, ok)
	}
}

func TestClassifyMedia(t *testing.T) {
	cases := []struct {
		mType string
		file  string
		want  string
	}{
		{"voice", "voice_123.ogg", MediaVoice},
		{"voice", "round_voice_123.mp4", MediaVideoNote},
		{"image", "photo.jpg", MediaImage},
		{"image", "sticker.webp", MediaSticker},
		{"document", "spec.pdf", MediaDocument},
		{"document", "song.mp3", MediaAudio},
		{"document", "demo.mp4", MediaVideo},
	}

	for _, c := range cases {
		got := ClassifyMedia(c.mType, c.file)
		if got != c.want {
			t.Errorf("ClassifyMedia(%q, %q) = %q, want %q", c.mType, c.file, got, c.want)
		}
	}
}

func TestCreateSkill(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "skills-create-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path, err := Create(tmpDir, "Security Audit")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	expectedPath := filepath.Join(tmpDir, "security-audit", FileName)
	if path != expectedPath {
		t.Errorf("expected path %q, got %q", expectedPath, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	sk, err := Parse(path, data)
	if err != nil {
		t.Fatalf("Parse created skill failed: %v", err)
	}
	if sk.Name != "security-audit" {
		t.Errorf("expected skill name 'security-audit', got %q", sk.Name)
	}
	if sk.Kind != KindSummary {
		t.Errorf("expected kind %q, got %q", KindSummary, sk.Kind)
	}
}

func TestImportSkillAndFinderAutoDiscovery(t *testing.T) {
	skillsDir := t.TempDir()
	extDir := t.TempDir()

	// 1. Import a plain Markdown file (without YAML frontmatter) from an external folder
	plainMdPath := filepath.Join(extDir, "Release Notes Digest.md")
	plainContent := "# Release Notes Digest\nSummarize all release blockers, PRs, and deployment timelines."
	if err := os.WriteFile(plainMdPath, []byte(plainContent), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := Import(skillsDir, plainMdPath)
	if err != nil {
		t.Fatalf("Import plain markdown failed: %v", err)
	}
	if len(imported) != 1 || imported[0].Name != "release-notes-digest" {
		t.Fatalf("expected imported skill 'release-notes-digest', got %+v", imported)
	}

	// 2. Simulate user dropping a standalone .md file directly into .agents/skills/ in Finder
	droppedMd := filepath.Join(skillsDir, "finder-quick-skill.md")
	if err := os.WriteFile(droppedMd, []byte("Extract all customer feature requests and urgency levels."), 0o644); err != nil {
		t.Fatal(err)
	}

	// 3. Simulate user duplicating a skill folder in Finder ("release-notes-digest copy") without changing frontmatter name
	dupDir := filepath.Join(skillsDir, "release-notes-digest copy")
	if err := os.MkdirAll(dupDir, 0o755); err != nil {
		t.Fatal(err)
	}
	origBytes, err := os.ReadFile(imported[0].Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dupDir, FileName), origBytes, 0o644); err != nil {
		t.Fatal(err)
	}

	list, errs := Load(skillsDir)
	if len(errs) > 0 {
		t.Fatalf("Load reported unexpected errors: %v", errs)
	}
	if _, ok := Find(list, "release-notes-digest"); !ok {
		t.Errorf("expected 'release-notes-digest' in loaded skills")
	}
	if _, ok := Find(list, "finder-quick-skill"); !ok {
		t.Errorf("expected standalone Finder file 'finder-quick-skill.md' to be normalized and loaded")
	}
	if _, ok := Find(list, "release-notes-digest-copy"); !ok {
		t.Errorf("expected duplicated Finder folder to be deduplicated as 'release-notes-digest-copy'")
	}
}
