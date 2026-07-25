// Command resumepdf prints the built resume page to a PDF that visitors can download.
//
//	hugo --gc --minify && go run ./tools/resumepdf/main.go
//
// It serves the already-built public/ directory on a local port, points a headless
// Chromium at the resume page, and writes the PDF back into public/ so it ships
// with the rest of the site.
//
// Printing the real page rather than rendering the resume a second way means the
// PDF can never drift from the site. The layout differences that matter for paper
// live in the @media print block of assets/ananke/css/alexis.css.
//
// Any Chromium will do. Set BROWSER to pick one explicitly, otherwise it looks for
// Chrome, Chromium and Edge in PATH and in the usual per-platform locations.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

func main() {
	var (
		dir     = flag.String("dir", "public", "built site to serve")
		page    = flag.String("page", "/resume/", "path to print")
		out     = flag.String("out", filepath.Join("public", "alexis-couvreur-resume.pdf"), "PDF to write")
		browser = flag.String("browser", "", "Chromium binary to use (defaults to $BROWSER, then autodetection)")
		timeout = flag.Duration("timeout", 90*time.Second, "give up after this long")
	)
	flag.Parse()

	if err := run(*dir, *page, *out, *browser, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "resumepdf: %v\n", err)
		os.Exit(1)
	}
}

func run(dir, page, out, browserFlag string, timeout time.Duration) error {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return fmt.Errorf("%s does not look like a built site, run `hugo --gc --minify` first", dir)
	}

	browser, err := findBrowser(browserFlag)
	if err != nil {
		return err
	}

	server, baseURL, err := serve(dir)
	if err != nil {
		return err
	}
	defer server.Close()

	absOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absOut), 0o755); err != nil {
		return err
	}
	// Chromium appends to nothing and reports success even when it wrote no file,
	// so start from a clean slate and treat "still missing" as a failure.
	os.Remove(absOut)

	// A throwaway profile keeps this from touching, or being blocked by, the
	// browser the user already has open.
	profile, err := os.MkdirTemp("", "resumepdf-profile-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(profile)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := baseURL + page
	args := []string{
		"--headless=new",
		"--disable-gpu",
		// Required on CI runners, harmless locally.
		"--no-sandbox",
		"--no-first-run",
		"--no-default-browser-check",
		"--user-data-dir=" + profile,
		// Drop the URL and timestamp Chromium stamps into the margins by default.
		"--no-pdf-header-footer",
		// Let webfonts and the avatar finish loading before the snapshot.
		"--virtual-time-budget=15000",
		"--run-all-compositor-stages-before-draw",
		"--print-to-pdf=" + absOut,
		url,
	}

	fmt.Printf("printing %s\n", url)
	cmd := exec.CommandContext(ctx, browser, args...)
	output, runErr := cmd.CombinedOutput()
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%s timed out after %s", filepath.Base(browser), timeout)
	}

	// Edge on Windows hands the work to a detached process and its launcher
	// returns in well under a second, long before the PDF exists. Some builds
	// also exit non-zero on a perfectly good run, so the file on disk is the
	// only trustworthy signal here.
	info, err := waitForPDF(ctx, absOut)
	if err != nil {
		if runErr != nil {
			return fmt.Errorf("%s failed: %w\n%s", filepath.Base(browser), runErr, output)
		}
		return fmt.Errorf("%s: %w\n%s", filepath.Base(browser), err, output)
	}
	if info.Size() < 1024 {
		return fmt.Errorf("PDF is only %d bytes, something went wrong\n%s", info.Size(), output)
	}

	fmt.Printf("wrote %s (%.0f KB) using %s\n", out, float64(info.Size())/1024, filepath.Base(browser))
	return nil
}

// waitForPDF blocks until the file exists and has stopped growing, so a partially
// flushed PDF is never mistaken for a finished one.
func waitForPDF(ctx context.Context, path string) (os.FileInfo, error) {
	var previousSize int64 = -1
	for {
		select {
		case <-ctx.Done():
			return nil, errors.New("no PDF was written before the deadline")
		case <-time.After(250 * time.Millisecond):
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

// serve publishes dir on a free loopback port. Chromium is pointed at HTTP rather
// than file:// so that absolute asset paths and the print stylesheet resolve the
// same way they do for a visitor.
func serve(dir string) (*http.Server, string, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, "", err
	}
	server := &http.Server{Handler: http.FileServer(http.Dir(dir))}
	go server.Serve(listener)

	return server, "http://" + listener.Addr().String(), nil
}

func findBrowser(explicit string) (string, error) {
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
