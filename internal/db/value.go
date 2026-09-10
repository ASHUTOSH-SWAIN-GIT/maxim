package db

import (
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// CellValue keeps database identity separate from terminal presentation.
// Raw byte slices are copied so their contents remain valid after rows.Scan.
type CellValue struct {
	Raw              any
	DatabaseTypeName string
	IsNull           bool
	Text             string
}

type DataRow []CellValue

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
	if cell.Text == "NULL" {
		return `"NULL"`
	}
	return display
}

func textCellValue(value string) CellValue {
	return CellValue{Raw: value, DatabaseTypeName: "TEXT", Text: value}
}
