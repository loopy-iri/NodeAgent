package agent

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"time"
)

// nodeRepo is the GitHub repo used for node self-updates.
const nodeRepo = "loopy-iri/NodeAgent"

var versionRe = regexp.MustCompile(`^v?[0-9][0-9A-Za-z.\-]{0,30}$`)

// --- core lifecycle (master scope) ---

// coreStop stops the shared Xray core (all customers drop until started again).
func (s *Server) coreStop(w http.ResponseWriter, _ *http.Request) {
	s.mgr.Stop()
	writeJSON(w, http.StatusOK, map[string]any{"core_started": s.mgr.Started()})
}

// coreStart starts the core from the current config (no-op if already running
// with the same config; re-applies users).
func (s *Server) coreStart(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.RestartCore(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"core_started": s.mgr.Started(), "core_version": s.mgr.Version()})
}

// coreRestart restarts the core with the current config.
func (s *Server) coreRestart(w http.ResponseWriter, r *http.Request) {
	if err := s.mgr.RestartCore(r.Context()); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"core_started": s.mgr.Started(), "core_version": s.mgr.Version()})
}

// --- Xray-core version change (master scope) ---

type xrayVersionRequest struct {
	Version string `json:"version"` // e.g. v1.8.23; "latest" for the newest
}

// setXrayVersion downloads the requested Xray-core release, replaces the node's
// xray binary and restarts the core.
func (s *Server) setXrayVersion(w http.ResponseWriter, r *http.Request) {
	var req xrayVersionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json body")
		return
	}
	version := req.Version
	if version == "" || version == "latest" {
		v, err := latestRelease("XTLS/Xray-core")
		if err != nil {
			writeError(w, http.StatusBadGateway, "resolve latest xray: "+err.Error())
			return
		}
		version = v
	}
	if !versionRe.MatchString(version) {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}
	exePath := s.mgr.XrayExecutablePath()
	if err := downloadXray(version, exePath); err != nil {
		writeError(w, http.StatusBadGateway, "download xray: "+err.Error())
		return
	}
	if err := s.mgr.RestartCore(r.Context()); err != nil {
		// Binary swapped but core couldn't restart (e.g. no config yet).
		writeJSON(w, http.StatusOK, map[string]any{"xray_version": version, "core_started": false, "warning": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"xray_version": version, "core_started": s.mgr.Started(), "core_version": s.mgr.Version()})
}

// --- node binary self-update (master scope) ---

type nodeUpdateRequest struct {
	Version string `json:"version"` // optional; default latest
}

// selfUpdate downloads the requested NodeAgent release, replaces this binary and
// restarts the systemd service so the new version takes over.
func (s *Server) selfUpdate(w http.ResponseWriter, r *http.Request) {
	var req nodeUpdateRequest
	_ = decodeJSON(r, &req)
	version := req.Version
	if version == "" || version == "latest" {
		v, err := latestRelease(nodeRepo)
		if err != nil {
			writeError(w, http.StatusBadGateway, "resolve latest release: "+err.Error())
			return
		}
		version = v
	}
	if !versionRe.MatchString(version) {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}
	exe, err := os.Executable()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "locate executable: "+err.Error())
		return
	}
	if err := downloadNodeBinary(version, exe); err != nil {
		writeError(w, http.StatusBadGateway, "download node binary: "+err.Error())
		return
	}
	// Restart the service in the background so the response is sent first. Use
	// systemd-run so the restart runs in its own transient scope and isn't killed
	// when this unit stops; fall back to setsid.
	unit := os.Getenv("PG_AGENT_SERVICE_NAME")
	if unit == "" {
		unit = "pg-node-agent"
	}
	go func() {
		time.Sleep(700 * time.Millisecond)
		if _, err := exec.LookPath("systemd-run"); err == nil {
			_ = exec.Command("systemd-run", "--no-block", "systemctl", "restart", unit).Start()
			return
		}
		_ = exec.Command("setsid", "systemctl", "restart", unit).Start()
	}()
	writeJSON(w, http.StatusOK, map[string]any{"updated_to": version, "restarting": true})
}

// --- helpers ---

func archTokens() (xrayArch, releaseArch string) {
	switch runtime.GOARCH {
	case "amd64":
		return "64", "amd64"
	case "arm64":
		return "arm64-v8a", "arm64"
	case "arm":
		return "arm32-v7a", "armv7"
	default:
		return "64", "amd64"
	}
}

func latestRelease(repo string) (string, error) {
	c := &http.Client{Timeout: 15 * time.Second}
	resp, err := c.Get("https://api.github.com/repos/" + repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	m := regexp.MustCompile(`"tag_name":\s*"([^"]+)"`).FindSubmatch(body)
	if m == nil {
		return "", fmt.Errorf("no release found for %s", repo)
	}
	return string(m[1]), nil
}

func httpDownload(url string) ([]byte, error) {
	c := &http.Client{Timeout: 120 * time.Second}
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// downloadXray fetches Xray-linux-<arch>.zip for version and writes the "xray"
// binary to dst (atomically replacing it).
func downloadXray(version, dst string) error {
	xrayArch, _ := archTokens()
	url := fmt.Sprintf("https://github.com/XTLS/Xray-core/releases/download/%s/Xray-linux-%s.zip", version, xrayArch)
	data, err := httpDownload(url)
	if err != nil {
		return err
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	for _, f := range zr.File {
		if filepath.Base(f.Name) != "xray" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()
		return writeBinaryAtomic(dst, rc)
	}
	return fmt.Errorf("xray binary not found in archive")
}

// downloadNodeBinary fetches the release tarball for version and replaces dst
// with the contained pg-node-agent binary.
func downloadNodeBinary(version, dst string) error {
	_, relArch := archTokens()
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/pg-node-agent_linux_%s.tar.gz", nodeRepo, version, relArch)
	data, err := httpDownload(url)
	if err != nil {
		return err
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) == "pg-node-agent" {
			return writeBinaryAtomic(dst, tr)
		}
	}
	return fmt.Errorf("pg-node-agent binary not found in archive")
}

// writeBinaryAtomic writes r to a temp file next to dst then renames over it.
func writeBinaryAtomic(dst string, r io.Reader) error {
	tmp := dst + ".new"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dst)
}
