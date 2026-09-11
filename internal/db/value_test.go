package db

import (
	"bytes"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func TestCellValueByteLimitKeepsUTF8ValidAndMarksPreview(t *testing.T) {
	cell := newCellValue([]byte(strings.Repeat("世界", 100)), "TEXT").bounded(64)
	if !cell.Truncated || cell.OriginalBytes <= 64 || cell.retainedBytes() > 64 || !utf8.ValidString(cell.Text) {
		t.Fatalf("cell was not safely bounded: %#v retained=%d", cell, cell.retainedBytes())
	}
	if !strings.Contains(cell.DisplayText(false), "truncated from") {
		t.Fatalf("truncation was hidden: %q", cell.DisplayText(false))
	}
}

func TestCellValueKeepsIdentitySeparateFromDisplay(t *testing.T) {
	nullCell := newCellValue(nil, "TEXT")
	textNull := newCellValue([]byte("NULL"), "TEXT")
	empty := newCellValue([]byte{}, "TEXT")
	if !nullCell.IsNull || nullCell.Raw != nil || nullCell.DisplayText(false) != "NULL" {
		t.Fatalf("SQL NULL identity was lost: %#v", nullCell)
	}
	if textNull.IsNull || textNull.Text != "NULL" || textNull.DisplayText(false) != `"NULL"` {
		t.Fatalf("text NULL was not distinguished: %#v", textNull)
	}
	if empty.IsNull || empty.DisplayText(false) != `""` {
		t.Fatalf("empty text was not distinguished: %#v", empty)
	}
}

func TestCellValuePreservesPrecisionTimeAndBytes(t *testing.T) {
	number := "99999999999999999999.12345678901234567890"
	numericCell := newCellValue([]byte(number), "NUMERIC")
	if numericCell.Text != number {
		t.Fatalf("numeric precision changed: %q", numericCell.Text)
	}

	moment := time.Date(2026, 9, 10, 12, 34, 56, 123456789, time.FixedZone("IST", 5*60*60+30*60))
	timeCell := newCellValue(moment, "TIMESTAMPTZ")
	if timeCell.Text != "2026-09-10T12:34:56.123456789+05:30" {
		t.Fatalf("timestamp identity changed: %q", timeCell.Text)
	}

	raw := []byte{0x00, 0x1b, 0xff}
	byteCell := newCellValue(raw, "BYTEA")
	raw[0] = 0xaa
	if !bytes.Equal(byteCell.Raw.([]byte), []byte{0x00, 0x1b, 0xff}) || byteCell.Text != `\x001bff` {
		t.Fatalf("byte value was not copied and encoded: %#v", byteCell)
	}
}

func TestCellValueEscapesTerminalControlsAndPreservesOptionalNewlines(t *testing.T) {
	cell := newCellValue("first\nsecond\t\x1b[31m世界", "TEXT")
	grid := cell.DisplayText(false)
	peek := cell.DisplayText(true)
	if strings.ContainsRune(grid, '\x1b') || strings.ContainsRune(peek, '\x1b') {
		t.Fatalf("terminal escape remained executable: grid=%q peek=%q", grid, peek)
	}
	if grid != `first\nsecond\t\x1b[31m世界` {
		t.Fatalf("unexpected grid display: %q", grid)
	}
	if peek != "first\nsecond\\t\\x1b[31m世界" {
		t.Fatalf("unexpected multiline display: %q", peek)
	}
}
