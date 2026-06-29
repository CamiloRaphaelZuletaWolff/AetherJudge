package api

import (
	"regexp"
	"strings"
	"time"
)

const (
	maxCodeBytes  = 256 * 1024
	maxStdinBytes = 1024 * 1024
	// maxBodyBytes caps any request body comfortably above code+stdin.
	maxBodyBytes = 2 * 1024 * 1024
	// maxAuthBodyBytes caps auth payloads, which are tiny by nature.
	maxAuthBodyBytes = 4 * 1024
	// maxStatementBytes caps a problem statement (markdown).
	maxStatementBytes = 64 * 1024
	// maxTestCaseFieldBytes caps each side of a test case (stdin / expected).
	maxTestCaseFieldBytes = 256 * 1024
	// maxAdminBodyBytes caps admin authoring payloads (statements + batched
	// test cases), comfortably above a statement plus several cases.
	maxAdminBodyBytes = 4 * 1024 * 1024
	// maxImportBytes caps an uploaded test-case file (ADR-0016).
	maxImportBytes = 8 * 1024 * 1024
	// maxImportCases caps how many test cases one uploaded file may yield.
	maxImportCases = 2000
)

var usernameRe = regexp.MustCompile(`^[A-Za-z0-9_]{3,32}$`)

func validateUsername(username string) *requestError {
	if !usernameRe.MatchString(username) {
		return badRequest("username must be 3-32 characters of letters, digits, or underscore")
	}
	return nil
}

func validateEmail(email string) *requestError {
	at := strings.Index(email, "@")
	if len(email) < 3 || len(email) > 254 || at < 1 || at == len(email)-1 {
		return badRequest("email address is not valid")
	}
	return nil
}

func validatePassword(password string) *requestError {
	if len(password) < 8 || len(password) > 72 {
		return badRequest("password must be between 8 and 72 bytes")
	}
	return nil
}

var validLanguages = map[string]bool{"cpp": true, "python": true, "go": true}

func validateLanguage(language string) *requestError {
	if !validLanguages[language] {
		return badRequest("language must be one of cpp, python, go")
	}
	return nil
}

func validateCode(code string) *requestError {
	if strings.TrimSpace(code) == "" {
		return badRequest("code must not be empty")
	}
	if len(code) > maxCodeBytes {
		return badRequest("code exceeds %d bytes", maxCodeBytes)
	}
	return nil
}

func validateStdin(stdin string) *requestError {
	if len(stdin) > maxStdinBytes {
		return badRequest("stdin exceeds %d bytes", maxStdinBytes)
	}
	return nil
}

// --- Admin content authoring (ADR-0015). These mirror the DB CHECK
// constraints in 00001_init.sql so the client sees errors before the round
// trip; the constraints remain the final backstop. ---

// slugRe matches the contests.slug CHECK: 3-64 chars, lowercase alphanumeric
// and hyphen, starting alphanumeric.
var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,63}$`)

func validateSlug(slug string) *requestError {
	if !slugRe.MatchString(slug) {
		return badRequest("slug must be 3-64 characters of lowercase letters, digits, or hyphens, starting with a letter or digit")
	}
	return nil
}

func validateContestTitle(title string) *requestError {
	n := len(strings.TrimSpace(title))
	if n < 1 || len(title) > 200 {
		return badRequest("title must be between 1 and 200 characters")
	}
	return nil
}

func validateContestTimes(startsAt, endsAt time.Time) *requestError {
	if !endsAt.After(startsAt) {
		return badRequest("the end time must be after the start time")
	}
	return nil
}

func validateStatement(statement string) *requestError {
	if strings.TrimSpace(statement) == "" {
		return badRequest("the problem statement must not be empty")
	}
	if len(statement) > maxStatementBytes {
		return badRequest("statement exceeds %d bytes", maxStatementBytes)
	}
	return nil
}

func validateTimeLimit(ms int) *requestError {
	if ms < 100 || ms > 10000 {
		return badRequest("time limit must be between 100 and 10000 ms")
	}
	return nil
}

func validateMemoryLimit(mb int) *requestError {
	if mb < 16 || mb > 512 {
		return badRequest("memory limit must be between 16 and 512 MB")
	}
	return nil
}

func validateTestCaseIO(stdin, expected string) *requestError {
	if len(stdin) > maxTestCaseFieldBytes || len(expected) > maxTestCaseFieldBytes {
		return badRequest("each test case field must be at most %d bytes", maxTestCaseFieldBytes)
	}
	return nil
}
