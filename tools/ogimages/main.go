// Command ogimages renders the social preview card for every page.
//
//	hugo --gc --minify && go run ./tools/ogimages
//
// Hugo emits a bare 1200x630 card at <page>/og.html through the "ogcard" output
// format. This walks the built site, screenshots each one to og.png beside it,
// and those PNGs are what the og:image and twitter:image tags point at.
//
// Cards are plain HTML with system fonts and no network calls, so a screenshot
// is deterministic and does not depend on a webfont arriving in time.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/acouvreur/personal-website/tools/internal/browser"
)

func main() {
	var (
		dir      = flag.String("dir", "public", "built site to walk")
		width    = flag.Int("width", 1200, "card width in pixels")
		height   = flag.Int("height", 630, "card height in pixels")
		bin      = flag.String("browser", "", "Chromium binary to use (defaults to $BROWSER, then autodetection)")
		keepHTML = flag.Bool("keep-html", false, "leave the og.html sources in the built site")
		timeout  = flag.Duration("timeout", 3*time.Minute, "give up after this long")
	)
	flag.Parse()

	if err := run(*dir, *width, *height, *bin, *keepHTML, *timeout); err != nil {
		fmt.Fprintf(os.Stderr, "ogimages: %v\n", err)
		os.Exit(1)
	}
}

func run(dir string, width, height int, bin string, keepHTML bool, timeout time.Duration) error {
	cards, err := findCards(dir)
	if err != nil {
		return err
	}
	if len(cards) == 0 {
		return fmt.Errorf("no og.html found under %s, run `hugo --gc --minify` first", dir)
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

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for _, card := range cards {
		output := filepath.Join(filepath.Dir(card), "og.png")
		absOutput, err := filepath.Abs(output)
		if err != nil {
			return err
		}

		args := append(browser.BaseArgs(profile),
			fmt.Sprintf("--window-size=%d,%d", width, height),
			"--screenshot="+absOutput,
			baseURL+urlPath(dir, card),
		)

		info, err := browser.Capture(ctx, chromium, args, absOutput)
		if err != nil {
			return fmt.Errorf("%s: %w", urlPath(dir, card), err)
		}
		fmt.Printf("  %-28s %5.0f KB\n", urlPath(dir, card), float64(info.Size())/1024)

		if !keepHTML {
			os.Remove(card)
		}
	}

	fmt.Printf("wrote %d social cards (%dx%d)\n", len(cards), width, height)
	return nil
}

func findCards(dir string) ([]string, error) {
	var cards []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && entry.Name() == "og.html" {
			cards = append(cards, path)
		}
		return nil
	})
	return cards, err
}

// urlPath turns public/resume/og.html into /resume/og.html.
func urlPath(dir, card string) string {
	rel, err := filepath.Rel(dir, card)
	if err != nil {
		return "/"
	}
	return "/" + filepath.ToSlash(rel)
}
