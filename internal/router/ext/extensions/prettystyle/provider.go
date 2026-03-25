package prettystyle

import (
	"fmt"
	"strings"

	"github.com/michaelbomholt665/wrlk/internal/router/capabilities"
)

const (
	ansiReset   = "\x1b[0m"
	ansiBold    = "\x1b[1m"
	ansiDim     = "\x1b[2m"
	ansiBlue    = "\x1b[34m"
	ansiGreen   = "\x1b[32m"
	ansiYellow  = "\x1b[33m"
	ansiRed     = "\x1b[31m"
	ansiMagenta = "\x1b[35m"
)

// Provider renders ANSI-styled text and ASCII tables.
type Provider struct{}

// StyleText returns styled terminal text for a supported kind.
func (p *Provider) StyleText(kind string, input string) (string, error) {
	switch kind {
	case "", capabilities.TextKindPlain:
		return input, nil
	case capabilities.TextKindHeader:
		return ansiBold + ansiMagenta + input + ansiReset, nil
	case capabilities.TextKindInfo:
		return ansiBold + ansiBlue + input + ansiReset, nil
	case capabilities.TextKindSuccess:
		return ansiBold + ansiGreen + input + ansiReset, nil
	case capabilities.TextKindWarning:
		return ansiBold + ansiYellow + input + ansiReset, nil
	case capabilities.TextKindError:
		return ansiBold + ansiRed + input + ansiReset, nil
	case capabilities.TextKindMuted:
		return ansiDim + input + ansiReset, nil
	default:
		return "", fmt.Errorf("style text: unsupported kind %q", kind)
	}
}

// StyleTable renders an ASCII table with consistent column widths.
func (p *Provider) StyleTable(headers []string, rows [][]string) (string, error) {
	if len(headers) == 0 {
		return "", fmt.Errorf("style table: headers must not be empty")
	}

	widths := make([]int, len(headers))
	for index, header := range headers {
		widths[index] = len(header)
	}

	for rowIndex, row := range rows {
		if len(row) != len(headers) {
			return "", fmt.Errorf(
				"style table: row %d width %d does not match header width %d",
				rowIndex,
				len(row),
				len(headers),
			)
		}

		for columnIndex, cell := range row {
			if len(cell) > widths[columnIndex] {
				widths[columnIndex] = len(cell)
			}
		}
	}

	var builder strings.Builder
	border := renderBorder(widths)
	builder.WriteString(border)
	builder.WriteString("\n")
	builder.WriteString(renderRow(headers, widths))
	builder.WriteString("\n")
	builder.WriteString(border)

	for _, row := range rows {
		builder.WriteString("\n")
		builder.WriteString(renderRow(row, widths))
	}

	builder.WriteString("\n")
	builder.WriteString(border)

	return builder.String(), nil
}

func renderBorder(widths []int) string {
	var builder strings.Builder
	builder.WriteString("+")
	for _, width := range widths {
		builder.WriteString(strings.Repeat("-", width+2))
		builder.WriteString("+")
	}

	return builder.String()
}

func renderRow(values []string, widths []int) string {
	var builder strings.Builder
	builder.WriteString("|")
	for index, value := range values {
		builder.WriteString(" ")
		builder.WriteString(value)
		builder.WriteString(strings.Repeat(" ", widths[index]-len(value)+1))
		builder.WriteString("|")
	}

	return builder.String()
}
