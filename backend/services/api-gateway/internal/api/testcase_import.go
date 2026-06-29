package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

// Bulk test-case import (ADR-0016). The server PARSES an uploaded file into
// (stdin, expected_output) pairs and returns them; it never writes to the DB.
// The frontend previews/edits the result and commits through the existing,
// validated batch endpoint (POST /admin/problems/{id}/test-cases).

// importCase is one parsed test case.
type importCase struct {
	Stdin          string
	ExpectedOutput string
}

// errUnsupportedFileType is returned for an unrecognized extension.
var errUnsupportedFileType = errors.New("unsupported file type")

// parseTestCases turns an uploaded file's bytes into test cases, dispatching on
// the lowercased extension. Supported: .csv, .xlsx, .json, .txt, .md.
func parseTestCases(filename string, data []byte) ([]importCase, error) {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".csv":
		return parseCSVCases(data)
	case ".xlsx":
		return parseXLSXCases(data)
	case ".json":
		return parseJSONCases(data)
	case ".txt", ".md":
		return parseBlockCases(data)
	default:
		return nil, errUnsupportedFileType
	}
}

// looksLikeHeader reports whether a tabular first row is a header to skip.
func looksLikeHeader(a, b string) bool {
	headers := map[string]bool{
		"input": true, "stdin": true,
		"output": true, "expected": true, "expected_output": true, "expected output": true,
	}
	norm := func(s string) string { return strings.ToLower(strings.TrimSpace(s)) }
	return headers[norm(a)] && headers[norm(b)]
}

func parseCSVCases(data []byte) ([]importCase, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1 // tolerate ragged rows; validated below
	rows, err := r.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("invalid CSV: %w", err)
	}
	var out []importCase
	for i, row := range rows {
		if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
			continue // blank line
		}
		if len(row) < 2 {
			return nil, fmt.Errorf("CSV row %d needs two columns (input, expected output)", i+1)
		}
		if i == 0 && looksLikeHeader(row[0], row[1]) {
			continue
		}
		out = append(out, importCase{Stdin: row[0], ExpectedOutput: row[1]})
	}
	return out, nil
}

func parseXLSXCases(data []byte) ([]importCase, error) {
	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("invalid XLSX: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return nil, errors.New("the spreadsheet has no sheets")
	}
	rows, err := f.GetRows(sheets[0])
	if err != nil {
		return nil, fmt.Errorf("read spreadsheet rows: %w", err)
	}
	var out []importCase
	for i, row := range rows {
		a, b := "", ""
		if len(row) > 0 {
			a = row[0]
		}
		if len(row) > 1 {
			b = row[1]
		}
		if strings.TrimSpace(a) == "" && strings.TrimSpace(b) == "" {
			continue // blank row
		}
		if i == 0 && looksLikeHeader(a, b) {
			continue
		}
		out = append(out, importCase{Stdin: a, ExpectedOutput: b})
	}
	return out, nil
}

func parseJSONCases(data []byte) ([]importCase, error) {
	var raw []struct {
		Stdin          *string `json:"stdin"`
		Input          *string `json:"input"`
		ExpectedOutput *string `json:"expected_output"`
		Output         *string `json:"output"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON: expected an array of {stdin, expected_output}: %w", err)
	}
	out := make([]importCase, 0, len(raw))
	for _, c := range raw {
		out = append(out, importCase{
			Stdin:          firstNonNil(c.Stdin, c.Input),
			ExpectedOutput: firstNonNil(c.ExpectedOutput, c.Output),
		})
	}
	return out, nil
}

func firstNonNil(a, b *string) string {
	if a != nil {
		return *a
	}
	if b != nil {
		return *b
	}
	return ""
}

// parseBlockCases parses the .txt/.md block format: a line that is only '='
// (>=3) separates cases; within a case a line that is only '-' (>=3) separates
// input from expected output. Internal newlines are preserved.
func parseBlockCases(data []byte) ([]importCase, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	var out []importCase
	for _, block := range splitOnSeparator(text, isRuleLine('=')) {
		if strings.TrimSpace(block) == "" {
			continue // skip blank/trailing separators
		}
		parts := splitOnSeparator(block, isRuleLine('-'))
		if len(parts) != 2 {
			return nil, errors.New("each test case must have exactly one '---' line separating input from expected output")
		}
		out = append(out, importCase{
			Stdin:          strings.Trim(parts[0], "\n"),
			ExpectedOutput: strings.Trim(parts[1], "\n"),
		})
	}
	return out, nil
}

// isRuleLine returns a predicate matching a line that, trimmed, is only the
// given rune repeated at least 3 times (e.g. "---", "====").
func isRuleLine(ch rune) func(string) bool {
	return func(line string) bool {
		t := strings.TrimSpace(line)
		return len(t) >= 3 && strings.Trim(t, string(ch)) == ""
	}
}

func splitOnSeparator(text string, isSep func(string) bool) []string {
	var segments []string
	var cur []string
	for _, ln := range strings.Split(text, "\n") {
		if isSep(ln) {
			segments = append(segments, strings.Join(cur, "\n"))
			cur = nil
			continue
		}
		cur = append(cur, ln)
	}
	return append(segments, strings.Join(cur, "\n"))
}

// parseTestCasesUpload accepts a multipart upload (field "file"), parses it into
// test cases, and returns them WITHOUT writing — the client saves them via the
// existing batch endpoint. Gated by problem.manage.
func (s *server) parseTestCasesUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxImportBytes)
	if err := r.ParseMultipartForm(maxImportBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			respondError(w, s.log, http.StatusRequestEntityTooLarge, "payload_too_large",
				fmt.Sprintf("file exceeds %d bytes", maxErr.Limit))
			return
		}
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "expected a multipart form with a 'file' field")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "missing 'file' field")
		return
	}
	defer func() { _ = file.Close() }()

	data, err := io.ReadAll(file)
	if err != nil {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "could not read the uploaded file")
		return
	}
	if len(data) == 0 {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "the uploaded file is empty")
		return
	}

	cases, err := parseTestCases(header.Filename, data)
	switch {
	case errors.Is(err, errUnsupportedFileType):
		respondError(w, s.log, http.StatusBadRequest, "unsupported_file_type",
			"unsupported file type; use .txt, .md, .csv, .json, or .xlsx")
		return
	case err != nil:
		respondError(w, s.log, http.StatusBadRequest, "parse_failed", err.Error())
		return
	}
	if len(cases) == 0 {
		respondError(w, s.log, http.StatusBadRequest, "bad_request", "no test cases found in the file")
		return
	}
	if len(cases) > maxImportCases {
		respondError(w, s.log, http.StatusBadRequest, "bad_request",
			fmt.Sprintf("too many test cases (%d); at most %d per file", len(cases), maxImportCases))
		return
	}
	for i, c := range cases {
		if v := validateTestCaseIO(c.Stdin, c.ExpectedOutput); v != nil {
			respondError(w, s.log, http.StatusBadRequest, "bad_request",
				fmt.Sprintf("test case %d: %s", i+1, v.message))
			return
		}
	}

	out := make([]testCaseDTO, len(cases))
	for i, c := range cases {
		out[i] = testCaseDTO{Ord: i + 1, Stdin: c.Stdin, ExpectedOutput: c.ExpectedOutput}
	}
	respondJSON(w, s.log, http.StatusOK, map[string]any{"cases": out, "count": len(out)})
}
