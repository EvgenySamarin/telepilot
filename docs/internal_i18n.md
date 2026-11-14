# Package `i18n`

```go
import "telepilot/internal/i18n"
```

## Constants

```go
const (
	En = "en"
	Ru = "ru"
)
```

## Functions

### `func IsRussian`

```go
func IsRussian(lang string) bool
```

IsRussian checks if the language code is Russian.

### `func Normalize`

```go
func Normalize(lang string) string
```

Normalize returns either "ru" or "en" (default).

### `func T`

```go
func T(lang, key string, args ...any) string
```

T translates a key for the given language.

