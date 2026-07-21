package config

import (
	"fmt"
	"strings"
)

// allowedTemplateTokens are the only placeholders permitted in a cloud-sync
// path template.
var allowedTemplateTokens = map[string]bool{
	"{year}":  true,
	"{month}": true,
	"{date}":  true,
	"{title}": true,
	"{ext}":   true,
	"{id}":    true,
}

// ValidateTemplate checks that tmpl only contains the allowed placeholder
// tokens ({year}, {month}, {date}, {title}, {ext}, {id}) and reports an error
// naming the first unknown token found. It is a pure function with no I/O and
// no dependency on Config; callers (e.g. the cloud-sync readiness probe)
// invoke it at check time, never during Load().
func ValidateTemplate(tmpl string) error {
	i := 0
	for i < len(tmpl) {
		open := strings.IndexByte(tmpl[i:], '{')
		if open == -1 {
			break
		}
		open += i
		close := strings.IndexByte(tmpl[open:], '}')
		if close == -1 {
			return fmt.Errorf("unterminated template token starting at %q", tmpl[open:])
		}
		close += open
		token := tmpl[open : close+1]
		if !allowedTemplateTokens[token] {
			return fmt.Errorf("unknown template token %q", token)
		}
		i = close + 1
	}
	return nil
}
