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

// bracketKeys are keywords whose value is a bracket-delimited list: "key: [a, b, c]".
var bracketKeys = map[string]bool{
	"on:":   true,
	"skip:": true,
}

// parseFields tokenizes a rule body into keyword fields and positional tokens.
//
// Single-pass algorithm: scan whitespace-separated tokens left-to-right.
// A token is a keyword key when ALL of these hold:
//   - It ends with ":"
//   - It is not a URL scheme token (does not contain "://")
//
// Special value handling per keyword type:
//   - bracketKeys (on:, skip:): consume the rest of the line up to and
//     including the closing "]", then continue scanning after it.
//   - cron:: if the next character is a double-quote, consume the quoted
//     string; otherwise consume tokens until the next keyword.
//   - multiwordKeys (unless:, undo:, after:): consume tokens until the
//     next keyword or end-of-input.
//   - all others: consume exactly one token.
//
// Tokens that are not keyword keys or keyword values are positional (f.tokens).
func parseFields(body string) lineFields {
	f := lineFields{
		kv: make(map[string]string),
	}

	// We work on the raw string with a cursor so that bracket and quoted
	// values (which may contain spaces) can be consumed without re-joining.
	s := strings.TrimSpace(body)

	for len(s) > 0 {
		s = strings.TrimSpace(s)
		if len(s) == 0 {
			break
		}

		// Find the end of the current token (next whitespace).
		spIdx := strings.IndexAny(s, " \t")
		var tok string
		if spIdx < 0 {
			tok = s
			s = ""
		} else {
			tok = s[:spIdx]
			s = strings.TrimSpace(s[spIdx:])
		}

		// Is this token a keyword key?
		// Condition: ends with ":" and contains no "://" (rules out URL schemes).
		if strings.HasSuffix(tok, ":") && !strings.Contains(tok, "://") {
			key := tok

			switch {
			case bracketKeys[key]:
				// Value is "[item, item, ...]" — may span the rest of the line.
				s = strings.TrimSpace(s)
				if strings.HasPrefix(s, "[") {
					end := strings.Index(s, "]")
					if end >= 0 {
						inner := s[1:end]
						s = strings.TrimSpace(s[end+1:])
						if key == "on:" {
							for _, part := range strings.Split(inner, ",") {
								if v := strings.TrimSpace(part); v != "" {
									f.osFilter = append(f.osFilter, v)
								}
							}
						} else {
							// skip: — store as comma-joined trimmed list
							var items []string
							for _, part := range strings.Split(inner, ",") {
								if v := strings.TrimSpace(part); v != "" {
									items = append(items, v)
								}
							}
							f.kv[key] = strings.Join(items, ",")
						}
					}
					// If no "]" found, ignore malformed bracket value.
				}
				// If not followed by "[", the keyword is silently ignored
				// (no value to extract, no side-effects on subsequent tokens).

			case key == "cron:":
				s = strings.TrimSpace(s)
				if strings.HasPrefix(s, `"`) {
					// Quoted cron expression: consume up to the closing quote.
					end := strings.Index(s[1:], `"`)
					if end >= 0 {
						f.kv[key] = s[1 : end+1]
						s = strings.TrimSpace(s[end+2:])
					}
				} else {
					// Unquoted: consume tokens until the next keyword or end.
					var parts []string
					for len(s) > 0 {
						sp := strings.IndexAny(s, " \t")
						var word string
						if sp < 0 {
							word = s
							s = ""
						} else {
							word = s[:sp]
							s = strings.TrimSpace(s[sp:])
						}
						if strings.HasSuffix(word, ":") && !strings.Contains(word, "://") {
							// Put the keyword back for the outer loop.
							s = word + " " + s
							break
						}
						parts = append(parts, word)
					}
					f.kv[key] = strings.Join(parts, " ")
				}

			case multiwordKeys[key]:
				// Consume tokens until the next keyword or end.
				var parts []string
				for len(s) > 0 {
					sp := strings.IndexAny(s, " \t")
					var word string
					if sp < 0 {
						word = s
						s = ""
					} else {
						word = s[:sp]
						s = strings.TrimSpace(s[sp:])
					}
					if strings.HasSuffix(word, ":") && !strings.Contains(word, "://") {
						s = word + " " + s
						break
					}
					parts = append(parts, word)
				}
				f.kv[key] = strings.Join(parts, " ")

			default:
				// Single-word value.
				sp := strings.IndexAny(s, " \t")
				if sp < 0 {
					f.kv[key] = s
					s = ""
				} else {
					f.kv[key] = s[:sp]
					s = strings.TrimSpace(s[sp:])
				}
			}
		} else {
			// Positional token.
			f.tokens = append(f.tokens, tok)
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
	// Strip optional bracket notation: "after: [a, b]" → "a, b"
	v = strings.TrimSpace(v)
	if strings.HasPrefix(v, "[") && strings.HasSuffix(v, "]") {
		v = v[1 : len(v)-1]
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
