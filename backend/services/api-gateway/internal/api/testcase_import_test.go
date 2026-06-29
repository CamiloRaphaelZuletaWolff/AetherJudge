package api

import (
	"bytes"
	"errors"
	"testing"

	"github.com/xuri/excelize/v2"
)

func assertCases(t *testing.T, got, want []importCase) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d cases, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("case %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestParseTestCasesBlock(t *testing.T) {
	t.Parallel()
	// CRLF, a trailing separator, and a multi-line case are all exercised.
	src := "1 2\r\n---\r\n3\r\n===\r\n10 20\n---\n30\n===\nmulti\nline\n---\nok\n===\n"
	got, err := parseTestCases("cases.txt", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertCases(t, got, []importCase{
		{"1 2", "3"},
		{"10 20", "30"},
		{"multi\nline", "ok"},
	})
}

func TestParseTestCasesCSV(t *testing.T) {
	t.Parallel()
	src := "input,output\n1 2,3\n\"a,b\",c\n"
	got, err := parseTestCases("cases.csv", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertCases(t, got, []importCase{{"1 2", "3"}, {"a,b", "c"}})
}

func TestParseTestCasesJSON(t *testing.T) {
	t.Parallel()
	src := `[{"stdin":"1 2","expected_output":"3"},{"input":"x","output":"y"}]`
	got, err := parseTestCases("cases.json", []byte(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertCases(t, got, []importCase{{"1 2", "3"}, {"x", "y"}})
}

func TestParseTestCasesXLSX(t *testing.T) {
	t.Parallel()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	for cell, v := range map[string]string{
		"A1": "input", "B1": "output",
		"A2": "1 2", "B2": "3",
		"A3": "7", "B3": "14",
	} {
		if err := f.SetCellValue(sheet, cell, v); err != nil {
			t.Fatalf("set %s: %v", cell, err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write xlsx: %v", err)
	}
	got, err := parseTestCases("cases.xlsx", buf.Bytes())
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	assertCases(t, got, []importCase{{"1 2", "3"}, {"7", "14"}})
}

func TestParseTestCasesErrors(t *testing.T) {
	t.Parallel()
	if _, err := parseTestCases("cases.bin", []byte("x")); !errors.Is(err, errUnsupportedFileType) {
		t.Errorf("unsupported ext err = %v, want errUnsupportedFileType", err)
	}
	if _, err := parseTestCases("cases.txt", []byte("1 2\n3\n")); err == nil {
		t.Error("block without --- should error")
	}
	if _, err := parseTestCases("cases.csv", []byte("only-one-col\n")); err == nil {
		t.Error("csv with one column should error")
	}
	if _, err := parseTestCases("cases.json", []byte("not json")); err == nil {
		t.Error("malformed json should error")
	}
}
