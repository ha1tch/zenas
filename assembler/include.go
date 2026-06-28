package assembler

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Include handling.
//
// zenas uses pasmo-style textual inclusion: an INCLUDE "file" line is replaced
// by the full text of the named file, assembled as if pasted at that point. The
// included file shares the single flat symbol namespace, and paths resolve
// relative to the file that contains the INCLUDE directive (not the working
// directory). This matches the existing pasmo source convention, where e.g.
// kernel.asm includes font_m3x6.asm by bare name.
//
// Expansion happens once, before lexing and before both assembly passes, so
// forward references across included files resolve naturally.

// maxIncludeDepth bounds transitive includes; primarily a guard against runaway
// recursion in addition to the explicit cycle detection below.
const maxIncludeDepth = 64

// expandIncludes recursively replaces INCLUDE lines in source with the contents
// of the referenced files. baseDir is the directory against which relative
// include paths are resolved for this source. visited tracks the absolute paths
// currently on the include stack, so a file that includes itself (directly or
// transitively) is reported as an error rather than looping.
func expandIncludes(source, baseDir string, visited []string, depth int) (string, error) {
	if depth > maxIncludeDepth {
		return "", fmt.Errorf("include nesting too deep (>%d); possible cycle", maxIncludeDepth)
	}

	lines := strings.Split(source, "\n")
	var out strings.Builder

	for _, line := range lines {
		incPath, ok := parseIncludeLine(line)
		if !ok {
			out.WriteString(line)
			out.WriteString("\n")
			continue
		}

		// Resolve relative to the including file's directory.
		resolved := incPath
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(baseDir, incPath)
		}
		abs, err := filepath.Abs(resolved)
		if err != nil {
			return "", fmt.Errorf("cannot resolve include path %q: %v", incPath, err)
		}

		// Cycle detection: is this file already on the include stack?
		for _, v := range visited {
			if v == abs {
				return "", fmt.Errorf("include cycle detected: %q is already being included", incPath)
			}
		}

		data, err := os.ReadFile(abs)
		if err != nil {
			return "", fmt.Errorf("cannot read included file %q: %v", incPath, err)
		}

		// Recurse with the included file's own directory as the new base, so
		// nested includes resolve relative to the file that names them.
		nestedBase := filepath.Dir(abs)
		expanded, err := expandIncludes(string(data), nestedBase, append(visited, abs), depth+1)
		if err != nil {
			return "", err
		}

		out.WriteString(expanded)
		if !strings.HasSuffix(expanded, "\n") {
			out.WriteString("\n")
		}
	}

	return out.String(), nil
}

// parseIncludeLine returns the quoted path if the line is an INCLUDE directive
// (bare or dotted), or ok=false otherwise. It tolerates leading whitespace, a
// trailing comment, and either quote style, but the path itself must be quoted -
// matching the existing source convention INCLUDE "file.asm".
func parseIncludeLine(line string) (path string, ok bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return "", false
	}

	// Strip a trailing line comment so INCLUDE "x" ; note is handled.
	if idx := strings.IndexByte(trimmed, ';'); idx >= 0 {
		trimmed = strings.TrimSpace(trimmed[:idx])
	}

	// Match the directive keyword case-insensitively, bare or dotted.
	upper := strings.ToUpper(trimmed)
	var rest string
	switch {
	case strings.HasPrefix(upper, ".INCLUDE"):
		rest = trimmed[len(".INCLUDE"):]
	case strings.HasPrefix(upper, "INCLUDE"):
		rest = trimmed[len("INCLUDE"):]
	default:
		return "", false
	}

	// The keyword must be followed by whitespace then a quoted path; this avoids
	// matching an identifier that merely starts with "include".
	if rest == "" || (rest[0] != ' ' && rest[0] != '\t') {
		return "", false
	}
	rest = strings.TrimSpace(rest)

	if len(rest) < 1 {
		return "", false
	}
	q := rest[0]
	if q == '"' || q == '\'' {
		end := strings.IndexByte(rest[1:], q)
		if end < 0 {
			return "", false
		}
		return rest[1 : 1+end], true
	}

	// Bare, unquoted filename (pasmo style: INCLUDE if.asm). Take the first
	// whitespace-delimited token as the path. The trailing comment was already
	// stripped above. This is unambiguous: native zenas source uses quoted
	// includes, so a bare path only appears in pasmo-style source.
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", false
	}
	return fields[0], true
}
