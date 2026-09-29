// Package fmedit edits structured frontmatter fields while preserving the
// document exactly: comments, key order, quoting style, and unrecognised
// fields all survive an edit to a recognised field, because only the named
// value's own line is touched — the rest of the document is never
// re-serialized. This is the one bounded round trip in the design; a raw
// text view remains the recovery path.
package fmedit

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/goccy/go-yaml"
)

// Update sets field to a scalar value in the frontmatter source. When the
// field is absent it is appended at document end (a new field, not a
// reordering). Lines other than the edited field's own line are returned
// byte-identical.
func Update(src []byte, field string, value any) ([]byte, error) {
	scalar, err := formatScalar(value)
	if err != nil {
		return nil, err
	}

	lineRe := regexp.MustCompile(`^(\s*` + regexp.QuoteMeta(field) + `:)(.*?)(\s+#.*)?$`)
	lines := splitLines(string(src))
	replaced := false

	for i, line := range lines {
		// Only top-level keys: no leading indentation, so a nested key of the
		// same name is never touched.
		if line != "" && (line[0] == ' ' || line[0] == '\t') {
			continue
		}
		m := lineRe.FindStringSubmatch(strings.TrimRight(line, "\r"))
		if m == nil {
			continue
		}
		valuePart := strings.TrimSpace(m[2])
		if valuePart == "|" || valuePart == ">" || strings.HasPrefix(valuePart, "|") || strings.HasPrefix(valuePart, ">") {
			return nil, fmt.Errorf("fmedit: field %q uses a block scalar; edit it in the raw view", field)
		}
		newLine := m[1] + " " + scalar
		if m[3] != "" { // preserve the author's trailing comment
			newLine += m[3]
		}
		lines[i] = newLine
		replaced = true
		break
	}

	if !replaced {
		out := strings.TrimRight(string(src), "\n")
		if out != "" {
			out += "\n"
		}
		out += field + ": " + scalar + "\n"
		return []byte(out), nil
	}
	eol := "\n"
	if strings.Contains(string(src), "\r\n") { // preserve the file's EOL style
		eol = "\r\n"
	}
	return []byte(strings.Join(lines, eol)), nil
}

// splitLines splits on the dominant line ending without keeping it.
func splitLines(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(s, "\n")
}

func formatScalar(value any) (string, error) {
	switch v := value.(type) {
	case string:
		if needsQuoting(v) {
			return quoteYAML(v), nil
		}
		return v, nil
	case bool, int, int64, float64:
		return fmt.Sprintf("%v", v), nil
	default:
		return "", fmt.Errorf("fmedit: unsupported field value type %T", value)
	}
}

// needsQuoting reports whether a string value would be ambiguous as a bare
// YAML scalar.
func needsQuoting(s string) bool {
	if s == "" {
		return true
	}
	if strings.TrimSpace(s) != s {
		return true
	}
	first := s[0]
	if strings.ContainsAny(s, ":#{}[]&*!|>'\"%@`") || first == '-' || first == '?' {
		return true
	}
	// Values that look like other scalar types must be quoted to stay strings.
	var probe any
	if err := yaml.Unmarshal([]byte("v: "+s), &probe); err != nil {
		return true
	}
	if m, ok := probe.(map[string]any); ok {
		if _, isStr := m["v"].(string); !isStr {
			return true
		}
	}
	return false
}

// quoteYAML renders a double-quoted scalar.
func quoteYAML(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

// Changed compares current frontmatter against values and reports which
// fields actually differ — the structured editor writes only what changed.
func Changed(src []byte, values map[string]any) map[string]any {
	current := map[string]any{}
	_ = yaml.Unmarshal(src, &current)
	changed := map[string]any{}
	for k, v := range values {
		cur, ok := current[k]
		if !ok || !scalarEqual(cur, v) {
			changed[k] = v
		}
	}
	return changed
}

func scalarEqual(a, b any) bool {
	return strings.TrimSpace(fmt.Sprintf("%v", a)) == strings.TrimSpace(fmt.Sprintf("%v", b))
}
