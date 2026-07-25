// Package browser drives a headless Chromium over the built site.
//
// Both the PDF and the social card generators work the same way: serve public/
// on a loopback port, point a browser at a page, and wait for a file to appear.
// The waiting is the fiddly part and is the main reason this is shared.
package browser

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

// Find locates a Chromium binary. Preference order: the explicit path, $BROWSER,
// anything on PATH, then the usual per-platform install locations.
func Find(explicit string) (string, error) {
	candidates := []string{explicit, os.Getenv("BROWSER")}
	candidates = append(candidates,
		"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
		"microsoft-edge", "microsoft-edge-stable", "msedge", "chrome",
	)
	candidates = append(candidates, platformPaths()...)

	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("no Chromium found: install Chrome, Chromium or Edge, or set BROWSER to its path")
}

func platformPaths() []string {
	switch runtime.GOOS {
	case "windows":
		var paths []string
		for _, root := range []string{
			os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA"),
		} {
			if root == "" {
				continue
			}
			paths = append(paths,
				filepath.Join(root, "Google", "Chrome", "Application", "chrome.exe"),
				filepath.Join(root, "Microsoft", "Edge", "Application", "msedge.exe"),
			)
		}
		return paths
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
		}
	default:
		return []string{
			"/usr/bin/google-chrome", "/usr/bin/chromium", "/usr/bin/chromium-browser",
			"/usr/bin/microsoft-edge", "/snap/bin/chromium",
		}
	}
}

// Serve publishes dir on a free loopback port and returns the server and its
// base URL. Browsers are pointed at HTTP rather than file:// so that absolute
// asset paths resolve the same way they do for a visitor.
func Serve(dir string) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	server := &http.Server{Handler: http.FileServer(http.Dir(dir))}
	go server.Serve(listener)

	return server, "http://" + listener.Addr().String(), nil
}

// BaseArgs are the switches every invocation needs. profile must be a directory
// that can be thrown away afterwards: without it the browser may refuse to start
// because the user already has one open, or worse, disturb their real profile.
func BaseArgs(profile string) []string {
	return []string{
		"--headless",
		"--disable-gpu",
		// Required on CI runners, harmless locally.
		"--no-sandbox",
		"--no-first-run",
		"--no-default-browser-check",
		"--hide-scrollbars",
		"--user-data-dir=" + profile,
	}
}

// Capture runs the browser and waits for output to appear and stop growing.
//
// Waiting is not optional. On Windows the launcher hands off to a detached
// process and returns in well under a second, long before the file exists, and
// some builds exit non-zero on a perfectly good run. The file on disk is the
// only trustworthy signal.
func Capture(ctx context.Context, bin string, args []string, output string) (os.FileInfo, error) {
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return nil, err
	}
	// Start clean so a stale file from a previous run cannot look like success.
	os.Remove(output)

	cmd := exec.CommandContext(ctx, bin, args...)
	combined, runErr := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return nil, fmt.Errorf("%s timed out", filepath.Base(bin))
	}

	info, err := waitForFile(ctx, output)
	if err != nil {
		if runErr != nil {
			return nil, fmt.Errorf("%s failed: %w\n%s", filepath.Base(bin), runErr, combined)
		}
		return nil, fmt.Errorf("%s: %w\n%s", filepath.Base(bin), err, combined)
	}
	return info, nil
}

func waitForFile(ctx context.Context, path string) (os.FileInfo, error) {
	var previousSize int64 = -1
	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%s was never written", filepath.Base(path))
		case <-time.After(200 * time.Millisecond):
		}

		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > 0 && info.Size() == previousSize {
			return info, nil
		}
		previousSize = info.Size()
	}
}

// TempProfile creates a throwaway browser profile directory.
func TempProfile() (string, func(), error) {
	dir, err := os.MkdirTemp("", "headless-profile-")
	if err != nil {
		return "", nil, err
	}
	return dir, func() { os.RemoveAll(dir) }, nil
}
