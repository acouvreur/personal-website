// Command contributions regenerates the upstream open source contributions that
// the resume renders, so those counts are never hand-maintained.
//
//	go run ./tools/contributions/main.go
//
// It asks the GitHub search API for the pull requests the user authored, drops
// the ones in repositories they own or co-own, aggregates what is left by
// repository, and writes data/opensource.json.
//
// PRs are counted in three buckets. "merged" is the plain case. "landed" covers
// upstreams that apply a change on their side and close the PR instead of
// merging it, which GitHub reports as closed-and-not-merged and which would
// otherwise read as rejected work; list those under -closed-counts and their
// closed PRs are counted as accepted. "open" is carried alongside and only
// affects inclusion if you pass -min-open.
//
// Restricting -closed-counts to named upstreams matters: everywhere else, a
// closed unmerged PR really was turned down, and should not be advertised.
//
// Editorial fields ("summary", "badge") are NOT generated. They are read back
// out of the existing file and carried over, so regenerating never destroys the
// prose. Every other field is replaced.
//
// Authentication: GITHUB_TOKEN or GH_TOKEN if set, otherwise `gh auth token`.
// Unauthenticated requests work but are rate limited to 10 searches per minute.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	searchEndpoint = "https://api.github.com/search/issues"
	perPage        = 100
	// The search API refuses to page past 1000 results.
	maxResults = 1000
)

// Coursework and hackathon repositories. They are genuine merged PRs, which is
// exactly why a count alone cannot filter them out. They outrank single-PR
// contributions to Docker or Kubernetes that carry far more weight on a resume.
const defaultExcludedRepos = "pns-soa-h/uberoo," +
	"ThomasGauthierr/TER," +
	"Thyvador/hackathon-1r-2021-front-end"

// Upstreams that take a change and apply it themselves, then close the PR.
// Accepts either an owner ("espressif", covering every repository they own) or
// one "owner/name". Only add an entry here when you have checked that a closed
// PR really did get incorporated.
const defaultClosedCounts = "espressif"

// Entry is one repository's worth of upstream work, as the Hugo template reads it.
type Entry struct {
	Name   string `json:"name"`
	Merged int    `json:"merged"`
	// Closed without being merged, but incorporated anyway. Only ever set for
	// repositories matched by -closed-counts.
	Landed int `json:"landed,omitempty"`
	Open   int `json:"open,omitempty"`
	// Carried over from the previous file rather than generated.
	Badge   string `json:"badge,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// Accepted is everything that made it in, however it got there.
func (e Entry) Accepted() int { return e.Merged + e.Landed }

type searchResponse struct {
	TotalCount        int  `json:"total_count"`
	IncompleteResults bool `json:"incomplete_results"`
	Items             []struct {
		RepositoryURL string `json:"repository_url"`
	} `json:"items"`
}

type options struct {
	user         string
	excludeOrgs  []string
	excludeRepos map[string]bool
	closedCounts map[string]bool
	out          string
	minMerged    int
	minOpen      int
}

func main() {
	var (
		user         = flag.String("user", "acouvreur", "GitHub user whose pull requests to collect")
		excludeOrgs  = flag.String("exclude-org", "reeveel,sablierapp", "comma-separated orgs to treat as own work, not upstream")
		excludeRepos = flag.String("exclude-repo", defaultExcludedRepos, "comma-separated owner/name repositories to omit")
		closedCounts = flag.String("closed-counts", defaultClosedCounts, "comma-separated owners or owner/name repositories where a closed PR counts as accepted")
		out          = flag.String("out", filepath.Join("data", "opensource.json"), "file to write")
		minMerged    = flag.Int("min-merged", 1, "minimum accepted PRs for a repository to be included")
		minOpen      = flag.Int("min-open", 0, "if above zero, also include repositories with at least this many open PRs")
	)
	flag.Parse()

	opts := options{
		user:         *user,
		excludeOrgs:  splitList(*excludeOrgs),
		excludeRepos: toSet(splitList(*excludeRepos)),
		closedCounts: toSet(splitList(*closedCounts)),
		out:          *out,
		minMerged:    *minMerged,
		minOpen:      *minOpen,
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "contributions: %v\n", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	merged, err := fetchCounts(opts, "is:merged")
	if err != nil {
		return fmt.Errorf("counting merged pull requests: %w", err)
	}
	// GitHub counts a merged PR as closed too, so this is a superset of merged.
	closed, err := fetchCounts(opts, "is:closed")
	if err != nil {
		return fmt.Errorf("counting closed pull requests: %w", err)
	}
	open, err := fetchCounts(opts, "is:open")
	if err != nil {
		return fmt.Errorf("counting open pull requests: %w", err)
	}
	if len(merged) == 0 && len(closed) == 0 && len(open) == 0 {
		return fmt.Errorf("no pull requests found for %q, refusing to write an empty file", opts.user)
	}

	entries := make([]Entry, 0, len(merged))
	for name := range union(merged, closed, open) {
		entry := Entry{Name: name, Merged: merged[name], Open: open[name]}
		// Everywhere else a closed unmerged PR was declined, so it stays uncounted.
		if opts.countsClosed(name) {
			entry.Landed = closed[name] - merged[name]
		}
		if !opts.includes(entry) {
			continue
		}
		entries = append(entries, entry)
	}

	// Busiest repositories first, alphabetical within a tie, so the file is
	// stable across runs and diffs stay readable.
	sort.Slice(entries, func(i, j int) bool {
		switch {
		case entries[i].Accepted() != entries[j].Accepted():
			return entries[i].Accepted() > entries[j].Accepted()
		case entries[i].Open != entries[j].Open:
			return entries[i].Open > entries[j].Open
		default:
			return entries[i].Name < entries[j].Name
		}
	})

	kept := carryOverProse(opts.out, entries)

	if err := writeJSON(opts.out, entries); err != nil {
		return err
	}

	totalMerged, totalLanded := totals(entries)
	fmt.Printf("wrote %s: %d repositories, %d merged PRs", opts.out, len(entries), totalMerged)
	if totalLanded > 0 {
		fmt.Printf(", %d landed without a merge", totalLanded)
	}
	if kept > 0 {
		fmt.Printf(", kept %d hand-written summaries", kept)
	}
	fmt.Println()
	return nil
}

func (o options) includes(e Entry) bool {
	if o.excludeRepos[e.Name] {
		return false
	}
	if e.Accepted() >= o.minMerged {
		return true
	}
	return o.minOpen > 0 && e.Open >= o.minOpen
}

// countsClosed reports whether this upstream closes PRs it has actually taken.
// Matches either the full "owner/name" or just the owner.
func (o options) countsClosed(repo string) bool {
	if o.closedCounts[repo] {
		return true
	}
	owner, _, found := strings.Cut(repo, "/")
	return found && o.closedCounts[owner]
}

// fetchCounts pages through the search API and tallies PRs per repository for
// one state qualifier, such as "is:merged".
func fetchCounts(opts options, state string) (map[string]int, error) {
	query := []string{
		"author:" + opts.user,
		"type:pr",
		state,
		// Contributions to one's own repositories are projects, not upstream work.
		"-user:" + opts.user,
	}
	for _, org := range opts.excludeOrgs {
		query = append(query, "-org:"+org)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := githubToken()
	counts := map[string]int{}

	for page := 1; (page-1)*perPage < maxResults; page++ {
		params := url.Values{}
		params.Set("q", strings.Join(query, " "))
		params.Set("per_page", fmt.Sprint(perPage))
		params.Set("page", fmt.Sprint(page))

		var result searchResponse
		if err := get(client, searchEndpoint+"?"+params.Encode(), token, &result); err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}

		for _, item := range result.Items {
			if repo := repoFromAPIURL(item.RepositoryURL); repo != "" {
				counts[repo]++
			}
		}

		if len(result.Items) < perPage {
			if result.TotalCount > maxResults {
				fmt.Fprintf(os.Stderr, "warning: %d results exceed the API's %d cap; counts may be low\n",
					result.TotalCount, maxResults)
			}
			break
		}
		// Stay clear of the secondary rate limit on consecutive searches.
		time.Sleep(2 * time.Second)
	}

	return counts, nil
}

func get(client *http.Client, endpoint, token string, into any) error {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "personal-website-contributions")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		hint := ""
		if resp.StatusCode == http.StatusForbidden && token == "" {
			hint = " (set GITHUB_TOKEN or run `gh auth login`, unauthenticated search is heavily rate limited)"
		}
		return fmt.Errorf("GitHub returned %s%s", resp.Status, hint)
	}
	return json.NewDecoder(resp.Body).Decode(into)
}

// carryOverProse copies summary and badge from the file being replaced, so the
// editorial text written by hand survives every regeneration. Returns how many
// entries kept a summary.
func carryOverProse(path string, entries []Entry) int {
	raw, err := os.ReadFile(path)
	if err != nil {
		return 0 // First run, or the file was removed on purpose.
	}
	var previous []Entry
	if err := json.Unmarshal(raw, &previous); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not parse existing %s, prose not carried over: %v\n", path, err)
		return 0
	}

	prose := make(map[string]Entry, len(previous))
	for _, entry := range previous {
		prose[entry.Name] = entry
	}

	kept := 0
	for i := range entries {
		old, ok := prose[entries[i].Name]
		if !ok {
			continue
		}
		entries[i].Summary = old.Summary
		entries[i].Badge = old.Badge
		if old.Summary != "" {
			kept++
		}
	}
	return kept
}

func writeJSON(path string, entries []Entry) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	encoded, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(encoded, '\n'), 0o644)
}

// repoFromAPIURL turns https://api.github.com/repos/owner/name into owner/name.
func repoFromAPIURL(apiURL string) string {
	parts := strings.Split(strings.TrimSuffix(apiURL, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-2] + "/" + parts[len(parts)-1]
}

// githubToken prefers the environment, then falls back to the gh CLI's token.
func githubToken() string {
	for _, key := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
		if token := strings.TrimSpace(os.Getenv(key)); token != "" {
			return token
		}
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func union(maps ...map[string]int) map[string]bool {
	all := map[string]bool{}
	for _, m := range maps {
		for key := range m {
			all[key] = true
		}
	}
	return all
}

func splitList(csv string) []string {
	var out []string
	for _, part := range strings.Split(csv, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func toSet(items []string) map[string]bool {
	set := make(map[string]bool, len(items))
	for _, item := range items {
		set[item] = true
	}
	return set
}

func totals(entries []Entry) (merged, landed int) {
	for _, entry := range entries {
		merged += entry.Merged
		landed += entry.Landed
	}
	return merged, landed
}
