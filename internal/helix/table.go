package helix

import (
	"fmt"
	"strings"
)

// LanguageRow is one language from the wide "hx --health languages" table.
// The table shows tool names only, never paths, and long names may be
// truncated with "…"; it is a candidate filter for doctor, not a full
// report. See docs/DECISIONS.md, D-006.
type LanguageRow struct {
	Language        string
	LanguageServers []Tool
	DebugAdapter    Tool
	Formatter       Tool
	Highlight       Status
	Textobjects     Status
	Indent          Status
}

// HasInstalledTool reports whether any language server, debug adapter or
// formatter shows ✓ in the row.
func (r LanguageRow) HasInstalledTool() bool {
	for _, t := range r.LanguageServers {
		if t.Status == StatusOK {
			return true
		}
	}
	return r.DebugAdapter.Status == StatusOK || r.Formatter.Status == StatusOK
}

// tableColumns is how many columns the table has: language, language
// servers, debug adapter, formatter, highlight, textobject, indent.
const tableColumns = 7

// Truncated is the suffix Helix appends when a cell does not fit.
const Truncated = "…"

// ParseTable converts the output of "hx --health languages" (or the table
// part of plain "hx --health") into rows. Helix lays the table out in
// equal-width columns (terminal width divided by the column count), so the
// width is read from where the second header cell starts and the terminal
// width Helix chose does not matter. In a narrow table Helix truncates
// cells with "…"; tool names lose the marker, language names keep it so
// callers can tell they are unusable. ExecRunner asks for a wide table
// (D-006), so this only matters for pasted or captured output.
func ParseTable(output string) ([]LanguageRow, error) {
	lines := strings.Split(StripANSI(output), "\n")

	// Find the header; plain "hx --health" prints clipboard info first.
	start := -1
	var offsets []int
	for i, l := range lines {
		width, ok := headerColumnWidth(l)
		if !ok {
			continue
		}
		for c := 0; c < tableColumns; c++ {
			offsets = append(offsets, c*width)
		}
		start = i
		break
	}
	if start < 0 {
		return nil, fmt.Errorf("%w: no language table header found", ErrUnexpectedOutput)
	}

	var rows []LanguageRow
	for _, l := range lines[start+1:] {
		if strings.TrimSpace(l) == "" {
			continue
		}
		cells := splitCells([]rune(l), offsets)
		lang := cells[0]
		if lang == "" {
			// Continuation row: another language server for the previous language.
			if len(rows) == 0 {
				return nil, fmt.Errorf("%w: continuation row before any language", ErrUnexpectedOutput)
			}
			if t, ok := parseTableTool(cells[1]); ok {
				rows[len(rows)-1].LanguageServers = append(rows[len(rows)-1].LanguageServers, t)
			}
			continue
		}
		row := LanguageRow{Language: lang}
		if t, ok := parseTableTool(cells[1]); ok {
			row.LanguageServers = append(row.LanguageServers, t)
		}
		row.DebugAdapter = tableSlot(cells[2])
		row.Formatter = tableSlot(cells[3])
		row.Highlight = parseMark(cells[4])
		row.Textobjects = parseMark(cells[5])
		row.Indent = parseMark(cells[6])
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%w: language table has no rows", ErrUnexpectedOutput)
	}
	return rows, nil
}

// headerColumnWidth recognises the header line ("Language" padded to the
// column width, then "Language servers" or a truncated "Language …") and
// returns the column width.
func headerColumnWidth(line string) (int, bool) {
	rest, ok := strings.CutPrefix(line, "Language ")
	if !ok {
		return 0, false
	}
	pad := len(rest) - len(strings.TrimLeft(rest, " "))
	width := len("Language ") + pad
	second := strings.TrimLeft(rest, " ")
	if pad == 0 || !strings.HasPrefix(second, "Language") {
		return 0, false
	}
	// The line is ASCII up to the second cell, so byte and rune offsets
	// agree for the width; a whole header needs room for every column.
	if len([]rune(line)) < (tableColumns-1)*width {
		return 0, false
	}
	return width, true
}

// splitCells slices a row into trimmed cells at the given rune offsets.
func splitCells(r []rune, offsets []int) []string {
	cells := make([]string, len(offsets))
	for i, start := range offsets {
		end := len(r)
		if i+1 < len(offsets) && offsets[i+1] < end {
			end = offsets[i+1]
		}
		if start >= len(r) {
			cells[i] = ""
			continue
		}
		cells[i] = strings.TrimSpace(string(r[start:end]))
	}
	return cells
}

// parseTableTool reads "✓ name" or "✘ name". "None" and blanks yield false.
func parseTableTool(cell string) (Tool, bool) {
	mark, name := cutMark(cell)
	if mark == "" {
		return Tool{}, false
	}
	name = strings.TrimSuffix(name, Truncated)
	t := Tool{Name: name, Binary: name}
	if mark == markOK {
		t.Status = StatusOK
	} else {
		t.Status = StatusMissing
	}
	return t, true
}

// tableSlot reads a debug adapter or formatter cell, where "None" means
// nothing configured.
func tableSlot(cell string) Tool {
	if t, ok := parseTableTool(cell); ok {
		return t
	}
	if cell == "None" {
		return Tool{Status: StatusNone}
	}
	return Tool{Binary: cell, Status: StatusUnknown}
}
