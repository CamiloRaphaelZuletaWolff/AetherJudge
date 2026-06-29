package integration

import (
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"testing"
)

func (s *stack) postFile(t *testing.T, path, token, filename string, content []byte) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.server.URL+path, &buf)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.server.Client().Do(req)
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

// TestTestCaseImportParse exercises the upload-parse endpoint (ADR-0016): an
// admin gets the parsed cases back (no DB write), and a plain user is refused.
func TestTestCaseImportParse(t *testing.T) {
	st := newStack(t)
	ctx := context.Background()

	adminName := uniqueName("import")
	admin := st.signup(t, adminName)
	if _, err := st.store.UpdateUserRole(ctx, mustUUID(t, admin.User.ID), "admin"); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	adminTok := st.login(t, adminName)

	csv := []byte("input,output\n1 2,3\n10 20,30\n")
	path := "/api/v1/admin/test-cases/parse"

	// A plain user cannot use the parser.
	plain := st.signup(t, uniqueName("importplain"))
	if code := st.postFile(t, path, plain.AccessToken, "cases.csv", csv); code.StatusCode != http.StatusForbidden {
		t.Errorf("plain user parse = %d, want 403", code.StatusCode)
		_ = code.Body.Close()
	} else {
		_ = code.Body.Close()
	}

	// Admin gets the parsed cases back.
	resp := st.postFile(t, path, adminTok, "cases.csv", csv)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("admin parse = %d, want 200", resp.StatusCode)
	}
	body := decodeBody[struct {
		Count int `json:"count"`
		Cases []struct {
			Stdin          string `json:"stdin"`
			ExpectedOutput string `json:"expected_output"`
		} `json:"cases"`
	}](t, resp)
	if body.Count != 2 || len(body.Cases) != 2 {
		t.Fatalf("parsed %d cases, want 2: %+v", body.Count, body.Cases)
	}
	if body.Cases[0].Stdin != "1 2" || body.Cases[0].ExpectedOutput != "3" {
		t.Errorf("first case = %+v, want {1 2, 3}", body.Cases[0])
	}
}
