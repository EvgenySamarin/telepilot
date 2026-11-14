# Package `main`

```go
import "telepilot/cmd/screens"
```

## Functions

### `func RenderTerminalSVG`

```go
func RenderTerminalSVG(viewStr string, widthCols, heightRows int, title string) string
```

RenderTerminalSVG converts an ANSI-styled terminal view string into a self-contained SVG terminal screenshot.

