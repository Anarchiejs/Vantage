package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const repoFileHeader = "# Repository paths for this vault, relative to the vault folder or absolute.\n# One Name=path per line. Vantage updates this file when you link a folder from the UI.\n"

func (s *Server) status(w http.ResponseWriter, r *http.Request) {
	row, err := s.q.Sources(r.Context())
	if failed(w, err) {
		return
	}
	res := Status{Repositories: row.Repositories, Notes: row.Notes, CanLinkRepos: s.linking != nil}
	if name, err := s.q.Setting(r.Context(), "vault_name"); err == nil {
		res.Vault = name
	}
	writeJSON(w, res)
}

// linkRepo records Name=path in <vault>/.vantage/repos, then reimports.
func (s *Server) linkRepo(w http.ResponseWriter, r *http.Request) {
	if s.linking == nil {
		writeError(w, http.StatusConflict, "start Vantage with \"vantage watch\" to link a folder from the UI")
		return
	}
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body LinkRepoRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 8<<10)).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	dir, err := checkRepoFolder(body.Path)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	project, err := s.q.ProjectByID(r.Context(), id)
	if failed(w, err) {
		return
	}
	if strings.ContainsAny(project.Name, "=\r\n") || strings.HasPrefix(strings.TrimSpace(project.Name), "#") {
		writeError(w, http.StatusBadRequest, "this project name cannot be written to the repository file; edit .vantage/repos by hand")
		return
	}
	if err := setRepoLine(s.linking.Vault, project.Name, dir); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if err := s.linking.Reimport(r.Context()); err != nil {
		writeError(w, http.StatusInternalServerError, "the folder was saved, but the import failed: "+err.Error())
		return
	}
	writeJSON(w, LinkRepoResponse{SourcePath: dir})
}

// checkRepoFolder accepts an existing absolute folder that contains .git.
func checkRepoFolder(raw string) (string, error) {
	path := strings.TrimSpace(raw)
	path = strings.Trim(path, `"'`)
	if path == "" || strings.ContainsAny(path, "\r\n\x00") {
		return "", errors.New("give the folder path of the repository")
	}
	if !filepath.IsAbs(path) {
		return "", errors.New("give an absolute path, for example C:\\Users\\me\\code\\atlas")
	}
	path = filepath.Clean(path)
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("folder not found: %s", path)
	}
	if _, err := os.Stat(filepath.Join(path, ".git")); err != nil {
		return "", fmt.Errorf("not a Git repository (no .git in %s)", path)
	}
	return path, nil
}

// setRepoLine replaces or appends "name=dir" in <vault>/.vantage/repos, keeping every other line.
// When only the previous .observatoire/repos exists, its lines are carried over.
func setRepoLine(vault, name, dir string) error {
	file := filepath.Join(vault, ".vantage", "repos")
	data, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		data, err = os.ReadFile(filepath.Join(vault, ".observatoire", "repos"))
		if errors.Is(err, os.ErrNotExist) {
			data, err = []byte(repoFileHeader), nil
		}
	}
	if err != nil {
		return err
	}
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	entry := name + "=" + dir
	replaced := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if key, _, ok := strings.Cut(trimmed, "="); ok && strings.TrimSpace(key) == name {
			if !replaced {
				lines[i] = entry
				replaced = true
			} else {
				lines[i] = "# " + line // a later duplicate would win over the new line
			}
		}
	}
	if !replaced {
		lines = append(lines, entry)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}
