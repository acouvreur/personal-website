// Command slides builds the talk decks in slides/ with Marp, into static/slides/
// where Hugo publishes them like any other static file.
//
//	npm ci && go run ./tools/slides
//
// Every deck is a directory slides/<slug>/ holding an index.md and whatever
// images it uses. Each one becomes:
//
//	static/slides/<slug>/index.html   the deck, to present in a browser
//	static/slides/<slug>/<slug>.pdf   one page per slide, no speaker notes
//	static/slides/<slug>/<slug>.pptx  one picture per slide, with the notes
//
// The deck directory is copied into place first and Marp runs on the copy, so a
// relative path in a deck or in the theme (images, ../fonts/) resolves the same
// way for the HTML a visitor opens as for the PDF and PowerPoint that Chromium
// renders. The theme's fonts are the site's own, copied from assets/fonts/ next
// to the decks, so slides and site cannot drift apart and no font service is
// involved.
//
// static/slides/ is generated and gitignored. It runs before Hugo, so that
// `hugo server` serves the decks too, and so the talks page can check which
// decks exist.
package main

import (
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/acouvreur/personal-website/tools/internal/browser"
)

func main() {
	var (
		src    = flag.String("src", "slides", "directory of decks, one per subdirectory")
		out    = flag.String("out", filepath.Join("static", "slides"), "where to write the built decks")
		themes = flag.String("themes", filepath.Join("slides", "themes"), "Marp theme directory")
		fonts  = flag.String("fonts", filepath.Join("assets", "fonts"), "where the theme's fonts live")
		marp   = flag.String("marp", filepath.Join("node_modules", "@marp-team", "marp-cli", "marp-cli.js"), "Marp CLI entry point, installed by `npm ci`")
		bin    = flag.String("browser", "", "Chromium binary to use (defaults to $BROWSER, then autodetection)")
	)
	flag.Parse()

	if err := run(*src, *out, *themes, *fonts, *marp, *bin); err != nil {
		fmt.Fprintf(os.Stderr, "slides: %v\n", err)
		os.Exit(1)
	}
}

func run(src, out, themes, fonts, marp, bin string) error {
	if _, err := os.Stat(marp); err != nil {
		return fmt.Errorf("%s not found, run `npm ci` first", marp)
	}
	chromium, err := browser.Find(bin)
	if err != nil {
		return err
	}

	decks, err := findDecks(src)
	if err != nil {
		return err
	}

	// Start clean, so a deck deleted from slides/ does not live on in the site.
	if err := os.RemoveAll(out); err != nil {
		return err
	}
	// Having no deck at all is a normal state between talks, not a failure.
	if len(decks) == 0 {
		fmt.Printf("slides: no decks in %s, nothing to build\n", src)
		return nil
	}
	if err := copyFonts(fonts, filepath.Join(out, "fonts")); err != nil {
		return err
	}

	for _, slug := range decks {
		dst := filepath.Join(out, slug)
		if err := copyDir(filepath.Join(src, slug), dst); err != nil {
			return fmt.Errorf("%s: %w", slug, err)
		}
		md := filepath.Join(dst, "index.md")

		builds := []struct {
			file  string
			flags []string
		}{
			{"index.html", nil},
			// No --pdf-notes: the PDF is for visitors, and notes would show up
			// in it as sticky-note icons. The PowerPoint keeps them.
			{slug + ".pdf", []string{"--pdf"}},
			{slug + ".pptx", []string{"--pptx"}},
		}
		for _, b := range builds {
			args := []string{marp, md,
				"--theme-set", themes,
				// Decks are our own Markdown, so inline HTML is safe, and it is
				// what the theme's diagram components (.layers) are built from.
				"--html",
				"--allow-local-files",
				"--browser-path", chromium,
				"--output", filepath.Join(dst, b.file),
			}
			args = append(args, b.flags...)
			cmd := exec.Command("node", args...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("%s: building %s: %w", slug, b.file, err)
			}
		}

		// The source has done its job; the site serves the outputs only.
		if err := os.Remove(md); err != nil {
			return err
		}
		fmt.Printf("slides: built %s\n", slug)
	}
	return nil
}

// findDecks returns the slug of every directory in src that holds an index.md.
// The themes directory has none, so it is skipped without a special case.
func findDecks(src string) ([]string, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return nil, err
	}
	var decks []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := os.Stat(filepath.Join(src, e.Name(), "index.md")); err == nil {
			decks = append(decks, e.Name())
		}
	}
	return decks, nil
}

// copyFonts copies the theme's font files next to the built decks.
func copyFonts(from, to string) error {
	matches, err := filepath.Glob(filepath.Join(from, "*.woff2"))
	if err != nil {
		return err
	}
	if len(matches) == 0 {
		return fmt.Errorf("no .woff2 fonts in %s", from)
	}
	if err := os.MkdirAll(to, 0o755); err != nil {
		return err
	}
	for _, f := range matches {
		if err := copyFile(f, filepath.Join(to, filepath.Base(f))); err != nil {
			return err
		}
	}
	return nil
}

func copyDir(from, to string) error {
	return filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(from, path)
		if err != nil {
			return err
		}
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasPrefix(d.Name(), ".") {
			return nil
		}
		return copyFile(path, target)
	})
}

func copyFile(from, to string) error {
	in, err := os.Open(from)
	if err != nil {
		return err
	}
	defer in.Close()
	outFile, err := os.Create(to)
	if err != nil {
		return err
	}
	if _, err := io.Copy(outFile, in); err != nil {
		outFile.Close()
		return err
	}
	return outFile.Close()
}
