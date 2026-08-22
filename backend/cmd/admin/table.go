package main

import (
	"io"
	"strings"
	"unicode"

	"golang.org/x/text/width"
)

type tableColumn struct {
	Title      string
	AlignRight bool
}

func writeTable(writer io.Writer, columns []tableColumn, rows [][]string) error {
	if len(columns) == 0 {
		return nil
	}
	widths := make([]int, len(columns))
	for i, column := range columns {
		widths[i] = terminalCellWidth(column.Title)
	}
	cleanRows := make([][]string, len(rows))
	for rowIndex, row := range rows {
		cleanRows[rowIndex] = make([]string, len(columns))
		for columnIndex := range columns {
			value := ""
			if columnIndex < len(row) {
				value = safeCell(row[columnIndex])
			}
			cleanRows[rowIndex][columnIndex] = value
			if cellWidth := terminalCellWidth(value); cellWidth > widths[columnIndex] {
				widths[columnIndex] = cellWidth
			}
		}
	}

	var output strings.Builder
	writeTableRow(&output, columns, widths, tableTitles(columns))
	for i, columnWidth := range widths {
		if i > 0 {
			output.WriteString("-+-")
		}
		output.WriteString(strings.Repeat("-", columnWidth))
	}
	output.WriteByte('\n')
	for _, row := range cleanRows {
		writeTableRow(&output, columns, widths, row)
	}
	_, err := io.WriteString(writer, output.String())
	return err
}

func writeTableRow(output *strings.Builder, columns []tableColumn, widths []int, values []string) {
	for i, column := range columns {
		if i > 0 {
			output.WriteString(" | ")
		}
		value := values[i]
		padding := strings.Repeat(" ", widths[i]-terminalCellWidth(value))
		if column.AlignRight {
			output.WriteString(padding)
			output.WriteString(value)
		} else {
			output.WriteString(value)
			output.WriteString(padding)
		}
	}
	output.WriteByte('\n')
}

func tableTitles(columns []tableColumn) []string {
	titles := make([]string, len(columns))
	for i, column := range columns {
		titles[i] = column.Title
	}
	return titles
}

// terminalCellWidth 按常见终端的实际列宽计算：中文、全角字符和 emoji 占两列，
// 组合符号不单独占列。这样不会出现 tabwriter 遇到中文后逐列偏移的问题。
func terminalCellWidth(value string) int {
	cellWidth := 0
	for _, item := range value {
		if unicode.IsControl(item) || unicode.Is(unicode.Mn, item) || unicode.Is(unicode.Me, item) || item == '\u200d' {
			continue
		}
		kind := width.LookupRune(item).Kind()
		if kind == width.EastAsianWide || kind == width.EastAsianFullwidth || isEmojiRune(item) {
			cellWidth += 2
		} else {
			cellWidth++
		}
	}
	return cellWidth
}

func isEmojiRune(item rune) bool {
	return item >= 0x1F300 && item <= 0x1FAFF ||
		item >= 0x2600 && item <= 0x26FF ||
		item >= 0x2700 && item <= 0x27BF
}
