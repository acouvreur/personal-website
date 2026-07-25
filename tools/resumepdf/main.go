// Command resumepdf prints the built resume page to a PDF that visitors can download.
//
//	hugo --gc --minify && go run ./tools/resumepdf
//
// It serves the already-built public/ directory on a local port, points a headless
// Chromium at the resume page, and writes the PDF back into public/ so it ships
// with the rest of the site.
//
// Printing the real page rather than rendering the resume a second way means the
// PDF can never drift from the site. The layout differences that matter for paper
// live in the @media print and @page blocks of assets/ananke/css/alexis.css.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/acouvreur/personal-website/tools/internal/browser"
)

func main() {
	var (
		dir     = flag.String("dir", "public", "built site to serve")
		page    = flag.String("page", "/resume/", "path to print")
		out     = flag.String("out", filepath.Join("public", "alexis-couvreur-resume.pdf"), "PDF to write")
		bin     = flag.String("browser", "", "Chromium binary to use (defaults to $BROWSER, then autodetection)")
		timeout = flag.Duration("timeout", 90*time.Second, "give up after this long")
	)
	flag.Parse()

	if err := run(*dir, *page, *out, *bin, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "resumepdf: %v\n", err)
		os.Exit(1)
	}
}

func run(dir, page, out, bin string, timeout time.Duration) error {
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		return fmt.Errorf("%s does not look like a built site, run `hugo --gc --minify` first", dir)
	}

	chromium, err := browser.Find(bin)
	if err != nil {
		return err
	}

	server, baseURL, err := browser.Serve(dir)
	if err != nil {
		return err
	}
	defer server.Close()

	profile, cleanup, err := browser.TempProfile()
	if err != nil {
		return err
	}
	defer cleanup()

	absOut, err := filepath.Abs(out)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := baseURL + page
	args := append(browser.BaseArgs(profile),
		// Drop the URL and timestamp Chromium stamps into the margins by default.
		"--no-pdf-header-footer",
		// Let the avatar and any webfonts finish before the snapshot.
		"--virtual-time-budget=15000",
		"--run-all-compositor-stages-before-draw",
		"--print-to-pdf="+absOut,
		url,
	)

	fmt.Printf("printing %s\n", url)
	info, err := browser.Capture(ctx, chromium, args, absOut)
	if err != nil {
		return err
	}
	if info.Size() < 1024 {
		return fmt.Errorf("PDF is only %d bytes, something went wrong", info.Size())
	}

	fmt.Printf("wrote %s (%.0f KB) using %s\n", out, float64(info.Size())/1024, filepath.Base(chromium))
	return nil
}
