# personal-website

Source of [www.alexiscouvreur.fr](https://www.alexiscouvreur.fr), a [Hugo](https://gohugo.io) site
serving both my resume and my blog.

## Layout

| Path | What it holds |
| --- | --- |
| `data/resume.json` | The resume, in [JSON Resume](https://jsonresume.org) schema. Hand-maintained. |
| `data/opensource.json` | Upstream contributions. **Generated**, see below. |
| `content/blog/` | Blog posts. |
| `layouts/` | Overrides on top of the [Ananke](https://github.com/theNewDynamic/gohugo-theme-ananke) theme (git submodule in `themes/`). |
| `assets/ananke/css/alexis.css` | Site styling, layered over the theme. Light and dark. |
| `assets/fonts/` | Self-hosted webfonts. Nothing loads from a CDN. |
| `tools/contributions/` | Generates the open source data and star counts. |
| `tools/ogimages/` | Screenshots the social preview cards. |
| `tools/resumepdf/` | Prints the resume page to a PDF. |

## Local development

Hugo is pinned in `mise.toml`, so [mise](https://mise.jdx.dev) gets you the exact version CI uses:

```sh
git submodule update --init --recursive
mise install
mise exec -- hugo server -D
```

Build the way CI does:

```sh
mise exec -- hugo --gc --minify
```

## Social preview cards

Every page carries its own `og:image`. Hugo renders a bare 1200x630 card at
`<page>/og.html` through the `ogcard` output format, and `tools/ogimages`
screenshots each one into `og.png` beside it before deleting the HTML.

The cards use system fonts and make no network calls, so a screenshot is
deterministic rather than a race against a webfont. Edit `layouts/all.ogcard.html`
to change the design, then `mise run dist` and open a card's `og.html` in a
browser to see it full size.

## The downloadable PDF

`/resume/` has a "Download as PDF" button pointing at `/alexis-couvreur-resume.pdf`.
That file is printed from the resume page itself by a headless browser:

```sh
mise run pdf      # builds the site, then prints the page
```

It serves `public/` on a local port, points Chromium at `/resume/`, and writes the
PDF back into `public/` so it ships inside the Pages artifact. CI does the same on
every push, using the Chrome preinstalled on the runner.

Printing the real page is the whole point: there is no second template to keep in
sync, so the PDF cannot drift from the site. How it looks on paper is controlled by
the `@media print` and `@page` blocks in `assets/ananke/css/alexis.css`. Anything
tagged `.ac-print-hide` (the download button itself) is dropped from the PDF, and
links stay live and clickable in it.

Any Chromium works. Set `BROWSER=/path/to/chrome` to choose one, otherwise it finds
Chrome, Chromium or Edge on `PATH` or in the usual places.

Note that the PDF only exists after a build, so the download button 404s under
`mise run dev`. Run `mise run pdf` once if you need it locally.

## Refreshing open source contributions

The "Upstream contributions" block on `/resume/` is generated from the GitHub API,
so the PR counts are never typed by hand:

```sh
mise run contributions      # or: go run ./tools/contributions/main.go
```

It skips repositories you own (`-user`) or co-own (`-exclude-org`) and writes two
files:

| File | Feeds | Contents |
| --- | --- | --- |
| `data/pullrequests.json` | `/opensource/` | Every pull request, one record each |
| `data/opensource.json` | `/resume/`, home page | Per-repository totals |

Two searches cover it: a closed PR carries `merged_at`, so `is:closed` tells merged
and declined apart on its own, and `is:open` brings in the rest.

PRs land in four states. **merged** is the plain case. **landed** is for upstreams
that apply a change on their side and close the PR instead of merging it, which
GitHub reports as closed-and-unmerged and would otherwise read as rejected work;
Espressif does this, and stamps each one with an internal tracker ID. **open**
speaks for itself, and **closed** is everything genuinely declined, which is shown
on `/opensource/` but counted nowhere.

Only repositories listed in `-closed-counts` get the landed treatment. Everywhere
else a closed unmerged PR really was declined.

Each PR also gets a `type` read off its conventional commit prefix (`fix(darwin):`
becomes `fix`), which is what the type filter on `/opensource/` runs on.

Two things it does **not** touch: the `summary` and `badge` fields. Those are
editorial, so the generator reads them out of the existing file and writes them
back. Regenerating never destroys prose you wrote.

Useful flags:

| Flag | Default | Effect |
| --- | --- | --- |
| `-exclude-repo` | coursework repos | Drop specific `owner/name` repositories. Counts alone can't filter these: a 6-PR school project outranks a 1-PR contribution to Docker. |
| `-closed-counts` | `espressif` | Owners or `owner/name` repositories where a closed PR counts as accepted. Only add one after checking the change really was incorporated. |
| `-min-merged` | `1` | Raise it to show only repositories you contributed to substantially. |
| `-min-open` | `0` | Set to `2` to also include repositories with open PRs and nothing accepted yet. |

Authentication uses `GITHUB_TOKEN`, then `GH_TOKEN`, then `gh auth token`. It works
unauthenticated too, but GitHub rate limits search to 10 requests per minute.

## Writing a post

```sh
mise exec -- hugo new content blog/my-post.md
```

New posts start as `draft = true`; flip it to `false` to publish. `/resume/` is print-styled, so
`Ctrl-P` on that page produces a clean PDF.

## Deployment

Pushing to `main` triggers [`.github/workflows/github-pages.yml`](.github/workflows/github-pages.yml),
which builds with Hugo, renders the social cards and the resume PDF, and publishes
to GitHub Pages. Pull requests run the build only, as a check.

[`.github/workflows/links.yml`](.github/workflows/links.yml) checks external links
with [lychee](https://lychee.cli.rs). It runs weekly rather than per-push, because
the resume points at fifty-odd third-party URLs that rot on their own schedule and
a dead devpost link should not block a deploy. A weekly failure opens one issue; on
a pull request that touches content it fails the check instead. Hosts that block
automated requests outright are excluded in `lychee.toml`.

## Theme and fonts

Colours are CSS custom properties, held in `assets/ananke/css/palette-*.css` and
selected by one entry in `ananke.custom_css`. `alexis.css` refers to the tokens
and never to a literal colour, so swapping the whole scheme is a one-line change.
Spare palettes (sand, blueprint, oscilloscope, spectrum) sit alongside the
active one.

The site is **light only**. Three of its four surfaces are printed artefacts, the
resume sheet, the PDF and the social cards, so a dark variant would have been the
odd one out rather than the default, and keeping one meant the resume flashing
white when a dark-mode visitor clicked into it.

`/resume/` is presented as a sheet of paper on a desk. That is scoped by
`body_classes = 'ac-paper'` in its front matter, which Ananke resolves per page,
so the treatment applies there and nowhere else.

Fonts are self-hosted from `assets/fonts/`: variable woff2 files covering the
whole weight range, split latin / latin-ext exactly as Google serves them. The
`@font-face` rules come from `layouts/_partials/fonts.html`, listed first in
`ananke.custom_css` so they land in the same stylesheet bundle. Nothing on the
site contacts a third party.
