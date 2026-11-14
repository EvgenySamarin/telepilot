# Package `skills`

```go
import "telepilot/internal/skills"
```

Package skills loads AI instruction "skills" from <project>/.agents/skills/<name>/SKILL.md.

A skill is a Markdown file with a small YAML-like frontmatter header:

	---
	name: period-summary
	description: Executive summary of a chat period
	kind: summary            # "summary" or "media"
	media: voice, audio      # media skills only: which media kinds it handles ("*" = fallback)
	---
	<instructions for the model>

The application reads the skill body and inlines it into the agy prompt, so skill
behaviour is deterministic and independent of agy's own skill discovery.

## Constants

```go
const (
	KindSummary = "summary"
	KindMedia   = "media"

	FileName = "SKILL.md"
)
```

Media kinds used to route media files to media skills.

```go
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
```

## Variables

AllMediaKinds lists every media kind in display order.

```go
var AllMediaKinds = []string{
	MediaVoice, MediaVideoNote, MediaAudio, MediaVideo, MediaImage, MediaSticker, MediaDocument,
}
```

## Functions

### `func ClassifyMedia`

```go
func ClassifyMedia(mediaType, fileName string) string
```

ClassifyMedia maps a stored media item (app-level media type + file name) to a media kind.

### `func Create`

```go
func Create(dir, name string) (string, error)
```

Create scaffolds a new summary skill from a template and returns its SKILL.md path.

### `func EnsureDefaults`

```go
func EnsureDefaults(dir string) error
```

EnsureDefaults writes the bundled default skills into dir if the directory is fresh
or has not been initialized yet. Once initialized, it creates a .initialized marker
so that user deletions are preserved and defaults are not resurrected on every restart.

### `func EnsureFrontmatter`

```go
func EnsureFrontmatter(fallbackSlug string, data []byte) []byte
```

EnsureFrontmatter wraps plain Markdown instructions in a default summary skill frontmatter
if the file does not already start with "---". If it starts with "---", it is returned as-is.

### `func ResetDefaults`

```go
func ResetDefaults(dir string) error
```

ResetDefaults restores all bundled default skills from defaultsFS into dir,
ensuring any missing or modified default files are restored,
while preserving any custom user-created skills.

### `func Slug`

```go
func Slug(name string) string
```

Slug normalizes a user-entered skill name into a folder-safe identifier.

## Types

### `type Skill`

Skill represents a parsed Markdown AI skill loaded from .agents/skills/<name>/SKILL.md.

```go
type Skill struct {
	Name        string
	Description string
	Kind        string
	Media       []string
	Path        string
	Body        string
	Hash        string // sha256 of the full SKILL.md content (first 12 hex chars)
}
```

#### `func Find`

```go
func Find(list []Skill, name string) (Skill, bool)
```

Find returns the skill with the given name.

#### `func ForMedia`

```go
func ForMedia(list []Skill, mediaKind string) (Skill, bool)
```

ForMedia returns the media skill handling the given media kind:
an exact match first, then a "*" fallback skill.

#### `func Import`

```go
func Import(destDir, srcPath string) ([]Skill, error)
```

Import imports one or more skills from srcPath (which may be a SKILL.md file, any .md file,
a skill directory containing SKILL.md, or a parent directory of skill folders) into destDir.

#### `func Load`

```go
func Load(dir string) (list []Skill, errs []error)
```

Load parses every <dir>/<name>/SKILL.md (and also normalizes standalone .md files
or single-.md folders added via macOS Finder). Broken skills are skipped and reported in errs.

#### `func Parse`

```go
func Parse(path string, data []byte) (Skill, error)
```

Parse parses SKILL.md content.

#### `func Summaries`

```go
func Summaries(list []Skill) []Skill
```

Summaries returns only summary skills.

