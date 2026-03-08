package parser

import (
	"fmt"
	"strings"
)

// lineFields holds the parsed keyword→value map and remaining positional tokens
// from a single rule line after the directive prefix is stripped.
type lineFields struct {
	kv       map[string]string // keyword → raw value string
	tokens   []string          // non-keyword positional tokens
	osFilter []string          // parsed from on: [...]
}

// multiwordKeys are keywords whose values may span multiple words (until the next keyword).
// All other keywords take exactly one word.
var multiwordKeys = map[string]bool{
	"unless:": true,
	"undo:":   true,
	"after:":  true, // comma-separated list which may contain spaces: "after: a, b, c"
}

// parseFields tokenizes a rule body into keyword fields and positional tokens.
//
// Algorithm:
//  1. Split off " on: [...]" from the right using strings.LastIndex to avoid
//     collisions with URLs that contain "on:" (e.g. "http://example.com/something").
//  2. Handle cron: "..." quoted value by extracting it before general tokenisation.
//  3. Handle skip: [...] bracket list similarly.
//  4. Scan remaining tokens left-to-right: a token ending in ":" is a keyword key;
//     the immediately following token(s) are its value.
//     - Single-word keywords: consume exactly one token.
//     - Multiword keywords (unless:, undo:): consume all tokens until the next keyword.
//  5. Tokens that are not keyword keys or keyword values are positional (f.tokens).
func parseFields(body string) lineFields {
	f := lineFields{
		kv: make(map[string]string),
	}

	// Step 1: extract "on: [...]" from the right.
	// Match " on:" (inline) or "on:" at position 0 (directive with no other tokens).
	var onRaw string
	if idx := strings.LastIndex(body, " on:"); idx >= 0 {
		onRaw = strings.TrimSpace(body[idx+4:])
		body = strings.TrimSpace(body[:idx])
	} else if strings.HasPrefix(body, "on:") {
		onRaw = strings.TrimSpace(body[3:])
		body = ""
	}
	if onRaw != "" {
		onValue := strings.Trim(onRaw, "[]")
		for _, part := range strings.Split(onValue, ",") {
			if s := strings.TrimSpace(part); s != "" {
				f.osFilter = append(f.osFilter, s)
			}
		}
	}

	// Step 2: extract cron: "..." quoted value before tokenisation to avoid
	// splitting on spaces inside the cron expression.
	if idx := strings.Index(body, `cron: "`); idx >= 0 {
		rest := body[idx+len(`cron: "`):]
		end := strings.Index(rest, `"`)
		if end >= 0 {
			f.kv["cron:"] = rest[:end]
			body = strings.TrimSpace(body[:idx]) + " " + strings.TrimSpace(rest[end+1:])
		}
	} else if idx := strings.Index(body, "cron:"); idx >= 0 {
		// unquoted cron: take until next keyword or end
		rest := strings.TrimSpace(body[idx+5:])
		body = strings.TrimSpace(body[:idx])
		// find next keyword boundary
		parts := strings.Fields(rest)
		var cronParts []string
		var remainder []string
		inCron := true
		for _, p := range parts {
			if inCron && strings.HasSuffix(p, ":") {
				inCron = false
				remainder = append(remainder, p)
			} else if inCron {
				cronParts = append(cronParts, p)
			} else {
				remainder = append(remainder, p)
			}
		}
		f.kv["cron:"] = strings.Join(cronParts, " ")
		if len(remainder) > 0 {
			body = body + " " + strings.Join(remainder, " ")
		}
	}

	// Step 3: extract skip: [...] bracket list.
	if idx := strings.Index(body, "skip:"); idx >= 0 {
		rest := strings.TrimSpace(body[idx+5:])
		body = strings.TrimSpace(body[:idx])
		if strings.HasPrefix(rest, "[") {
			end := strings.Index(rest, "]")
			if end >= 0 {
				inner := rest[1:end]
				var skipList []string
				for _, s := range strings.Split(inner, ",") {
					if v := strings.TrimSpace(s); v != "" {
						skipList = append(skipList, v)
					}
				}
				f.kv["skip:"] = strings.Join(skipList, ",")
				// anything after "]" goes back to body
				after := strings.TrimSpace(rest[end+1:])
				if after != "" {
					body = body + " " + after
				}
			}
		}
	}

	// Step 4 & 5: tokenise the remaining body.
	// after: is special: comma-separated list, but still single "word" in the token sense
	// (no spaces around commas). We treat it as a single-word keyword.
	tokens := strings.Fields(body)
	i := 0
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasSuffix(tok, ":") {
			key := tok
			i++
			if multiwordKeys[key] {
				// Consume tokens until the next keyword or end.
				var valueParts []string
				for i < len(tokens) && !strings.HasSuffix(tokens[i], ":") {
					valueParts = append(valueParts, tokens[i])
					i++
				}
				f.kv[key] = strings.Join(valueParts, " ")
			} else {
				// Single-word value.
				if i < len(tokens) {
					f.kv[key] = tokens[i]
					i++
				} else {
					f.kv[key] = ""
				}
			}
		} else {
			f.tokens = append(f.tokens, tok)
			i++
		}
	}

	return f
}

// word returns the first whitespace-separated word for a keyword, or "" if absent.
func (f lineFields) word(key string) string {
	v, ok := f.kv[key]
	if !ok {
		return ""
	}
	fields := strings.Fields(v)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

// multiword returns the full (possibly multi-word) value for a keyword, or "" if absent.
func (f lineFields) multiword(key string) string {
	return strings.TrimSpace(f.kv[key])
}

// list returns a comma-separated slice for a keyword (e.g. after:).
// Trims whitespace from each element.
func (f lineFields) list(key string) []string {
	v, ok := f.kv[key]
	if !ok {
		return nil
	}
	var result []string
	for _, part := range strings.Split(v, ",") {
		if s := strings.TrimSpace(part); s != "" {
			result = append(result, s)
		}
	}
	return result
}

// skipList returns the parsed skip: bracket list as a slice.
func (f lineFields) skipList() []string {
	return f.list("skip:")
}

// rest returns all positional (non-keyword) tokens joined by a single space.
func (f lineFields) rest() string {
	return strings.Join(f.tokens, " ")
}

// lineError formats a parse error with the offending line content.
func lineError(line, msg string) error {
	return fmt.Errorf("%s (in: %q)", msg, line)
}

// stripComment removes an inline comment from a line.
// Both # and // are supported as comment markers.
// A marker is only treated as a comment if it is preceded by whitespace or
// appears at the start of the line, so that URLs (https://) and values
// containing # (e.g. colour codes) are left intact.
func stripComment(line string) string {
	for i := 0; i < len(line); i++ {
		switch {
		case line[i] == '#':
			// # at start or preceded by whitespace → comment
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i]
			}
		case i+1 < len(line) && line[i] == '/' && line[i+1] == '/':
			// // at start or preceded by whitespace → comment
			if i == 0 || line[i-1] == ' ' || line[i-1] == '\t' {
				return line[:i]
			}
		}
	}
	return line
}
