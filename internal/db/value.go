package db

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// CellValue keeps database identity separate from terminal presentation.
// Raw byte slices are copied so their contents remain valid after rows.Scan.
type CellValue struct {
	Raw              any
	DatabaseTypeName string
	IsNull           bool
	Text             string
	Truncated        bool
	OriginalBytes    int
}

func (cell CellValue) retainedBytes() int {
	size := len(cell.Text)
	switch value := cell.Raw.(type) {
	case []byte:
		size += len(value)
	case string:
		size += len(value)
	}
	return size
}

func (cell CellValue) bounded(byteLimit int) CellValue {
	if byteLimit <= 0 || cell.IsNull || cell.retainedBytes() <= byteLimit {
		return cell
	}
	cell.OriginalBytes = cell.retainedBytes()
	cell.Truncated = true
	textLimit := max(byteLimit/2, 1)
	if len(cell.Text) > textLimit {
		end := textLimit
		for end > 0 && !utf8.RuneStart(cell.Text[end]) {
			end--
		}
		cell.Text = cell.Text[:end]
	}
	if value, ok := cell.Raw.([]byte); ok {
		rawLimit := max(byteLimit-len(cell.Text), 0)
		if len(value) > rawLimit {
			cell.Raw = append([]byte(nil), value[:rawLimit]...)
		}
	} else if value, ok := cell.Raw.(string); ok {
		rawLimit := max(byteLimit-len(cell.Text), 0)
		if len(value) > rawLimit {
			cell.Raw = value[:rawLimit]
		}
	}
	return cell
}

type DataRow []CellValue

func (cell CellValue) Clone() CellValue {
	cloned := cell
	if value, ok := cell.Raw.([]byte); ok {
		cloned.Raw = append([]byte(nil), value...)
	}
	return cloned
}

func newCellValue(raw any, databaseTypeName string) CellValue {
	cell := CellValue{DatabaseTypeName: strings.ToUpper(databaseTypeName), IsNull: raw == nil}
	if raw == nil {
		return cell
	}
	switch value := raw.(type) {
	case []byte:
		copied := append([]byte(nil), value...)
		cell.Raw = copied
		if cell.DatabaseTypeName == "BYTEA" {
			cell.Text = `\x` + hex.EncodeToString(copied)
		} else {
			cell.Text = string(copied)
		}
	case time.Time:
		cell.Raw = value
		cell.Text = value.Format(time.RFC3339Nano)
	case float32:
		cell.Raw = value
		cell.Text = strconv.FormatFloat(float64(value), 'g', -1, 32)
	case float64:
		cell.Raw = value
		cell.Text = strconv.FormatFloat(value, 'g', -1, 64)
	default:
		cell.Raw = raw
		cell.Text = fmt.Sprint(raw)
	}
	return cell
}

// DisplayText returns terminal-safe text. Multiline mode preserves newlines
// for row peek; the grid uses an explicit \n marker instead.
func (cell CellValue) DisplayText(multiline bool) string {
	if cell.IsNull {
		return "NULL"
	}
	if cell.Text == "" {
		return `""`
	}
	var output strings.Builder
	for _, character := range cell.Text {
		switch character {
		case '\n':
			if multiline {
				output.WriteRune('\n')
			} else {
				output.WriteString(`\n`)
			}
		case '\r':
			output.WriteString(`\r`)
		case '\t':
			output.WriteString(`\t`)
		default:
			if unicode.IsControl(character) {
				if character <= 0xff {
					fmt.Fprintf(&output, `\x%02x`, character)
				} else {
					fmt.Fprintf(&output, `\u%04x`, character)
				}
			} else {
				output.WriteRune(character)
			}
		}
	}
	display := output.String()
	if cell.Truncated {
		display += fmt.Sprintf("… [truncated from %d bytes]", cell.OriginalBytes)
	}
	if cell.Text == "NULL" {
		return `"NULL"`
	}
	return display
}

func textCellValue(value string) CellValue {
	return CellValue{Raw: value, DatabaseTypeName: "TEXT", Text: value}
}
