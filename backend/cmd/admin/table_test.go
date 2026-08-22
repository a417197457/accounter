package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestTerminalCellWidth(t *testing.T) {
	tests := map[string]int{
		"abc": 3,
		"账小鸭": 6,
		"🍱":   2,
	}
	for value, expected := range tests {
		if actual := terminalCellWidth(value); actual != expected {
			t.Fatalf("terminalCellWidth(%q)=%d, want %d", value, actual, expected)
		}
	}
}

func TestWriteTableAlignsChineseCells(t *testing.T) {
	var output bytes.Buffer
	err := writeTable(&output, []tableColumn{
		{Title: "用户ID", AlignRight: true},
		{Title: "昵称"},
		{Title: "金额", AlignRight: true},
	}, [][]string{
		{"1", "张三", "12.00"},
		{"20", "Alice🍱", "8.50"},
	})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(output.String()), "\n")
	if len(lines) != 4 {
		t.Fatalf("unexpected table:\n%s", output.String())
	}
	for lineNumber, line := range []string{lines[0], lines[2], lines[3]} {
		parts := strings.Split(line, " | ")
		if len(parts) != 3 {
			t.Fatalf("line %d has unexpected columns: %q", lineNumber, line)
		}
		if terminalCellWidth(parts[0]) != 6 || terminalCellWidth(parts[1]) != 7 {
			t.Fatalf("line %d is not aligned: %q", lineNumber, line)
		}
	}
}
