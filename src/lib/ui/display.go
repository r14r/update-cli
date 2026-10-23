package ui

import (
	"os"
	"path/filepath"
	"strings"
)

// DisplayText shortens absolute paths below the current user's home directory
// for presentation only. It never changes paths used for filesystem access.
//
// Example:
//
//	/Users/alice/Downloads/app-v1.2.3.zip -> $HOME/Downloads/app-v1.2.3.zip
func DisplayText(text string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return text
	}
	return replaceHomePath(text, home)
}

func replaceHomePath(text, home string) string {
	home = filepath.Clean(strings.TrimSpace(home))
	if text == "" || home == "" || home == "." || filepath.Dir(home) == home {
		return text
	}

	var out strings.Builder
	remaining := text
	for {
		idx := strings.Index(remaining, home)
		if idx < 0 {
			out.WriteString(remaining)
			break
		}

		out.WriteString(remaining[:idx])
		after := idx + len(home)
		beforeOK := idx == 0 || isHomePathLeadingBoundary(remaining[idx-1])
		afterOK := after == len(remaining) || isHomePathTrailingBoundary(remaining[after])
		if beforeOK && afterOK {
			out.WriteString("$HOME")
			remaining = remaining[after:]
			continue
		}

		// A lexical prefix such as /Users/alice-other is not inside the home
		// directory and must remain unchanged.
		out.WriteString(remaining[idx : idx+len(home)])
		remaining = remaining[after:]
	}
	return out.String()
}

func isHomePathLeadingBoundary(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == '=' || b == ':' || b == '(' || b == '[' || b == '{' || b == '\'' || b == '"'
}

func isHomePathTrailingBoundary(b byte) bool {
	return b == '/' || b == '\\' || b == ' ' || b == '\t' || b == '\n' || b == '\r' || b == ',' || b == ';' || b == ':' || b == ')' || b == ']' || b == '}' || b == '\'' || b == '"'
}
