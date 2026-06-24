package api

import (
	"regexp"
	"strings"
)

const (
	maxCodeBytes  = 256 * 1024
	maxStdinBytes = 1024 * 1024
	// maxBodyBytes caps any request body comfortably above code+stdin.
	maxBodyBytes = 2 * 1024 * 1024
	// maxAuthBodyBytes caps auth payloads, which are tiny by nature.
	maxAuthBodyBytes = 4 * 1024
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
