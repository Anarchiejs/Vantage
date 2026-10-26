package git

import (
	"path"
	"strings"
)

var langages = map[string]string{
	".go": "Go", ".ts": "TypeScript", ".tsx": "TSX", ".js": "JavaScript", ".jsx": "JavaScript",
	".mjs": "JavaScript", ".cjs": "JavaScript", ".sql": "SQL", ".md": "Markdown", ".markdown": "Markdown",
	".cs": "C#", ".cpp": "C++", ".cc": "C++", ".cxx": "C++", ".hpp": "C++", ".h": "C++",
	".py": "Python", ".rs": "Rust", ".lua": "Lua", ".luau": "Luau", ".ps1": "PowerShell", ".psm1": "PowerShell",
	".psd1": "PowerShell", ".yml": "YAML", ".yaml": "YAML", ".json": "JSON", ".html": "HTML", ".htm": "HTML", ".css": "CSS",
}

func langage(chemin string) string { return langages[strings.ToLower(path.Ext(chemin))] }
