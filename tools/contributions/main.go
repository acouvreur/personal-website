// Command contributions regenerates the open source contribution data the site
// renders, so none of it is hand-maintained.
//
//	go run ./tools/contributions/main.go
//
// It asks the GitHub search API for the pull requests the user authored, drops
// the ones in repositories they own or co-own, and writes two files:
//
//	data/pullrequests.json   every pull request, for the /opensource/ page
//	data/opensource.json     per-repository totals, for the resume
//
// Two searches cover it. A closed PR carries merged_at, so "is:closed" tells
// merged and declined apart on its own, and "is:open" brings in the rest.
//
// PRs are classified in four states. "merged" is the plain case. "landed" covers
// upstreams that apply a change on their side and close the PR instead of
// merging it, which GitHub reports as closed-and-unmerged and which would
// otherwise read as rejected work; list those under -closed-counts. "open"
// speaks for itself, and "closed" is everything genuinely declined.
//
// Restricting -closed-counts to named upstreams matters: everywhere else, a
// closed unmerged PR really was turned down, and should not be counted as work
// that shipped.
//
// Editorial fields ("summary", "badge") in data/opensource.json are NOT
// generated. They are read back out of the existing file and carried over, so
// regenerating never destroys the prose. Every other field is replaced.
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
	"regexp"
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

// Coursework and hackathon repositories. They hold genuine merged PRs, which is
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

// Pull request states, most to least favourable.
const (
	stateMerged = "merged"
	stateLanded = "landed"
	stateOpen   = "open"
	stateClosed = "closed"
)

// conventionalPrefix pulls the type out of titles like "fix(darwin): ..." or
// "feat!: ...". Anything that does not follow the convention becomes "other".
var conventionalPrefix = regexp.MustCompile(`^([a-zA-Z]+)(\([^)]*\))?!?:`)

var knownTypes = map[string]bool{
	"feat": true, "fix": true, "docs": true, "refactor": true, "test": true,
	"chore": true, "perf": true, "build": true, "ci": true, "style": true,
	"revert": true,
}

// PullRequest is one contribution, as the /opensource/ page reads it.
type PullRequest struct {
	Repo    string `json:"repo"`
	Number  int    `json:"number"`
	Title   string `json:"title"`
	URL     string `json:"url"`
	State   string `json:"state"`
	Type    string `json:"type"`
	Created string `json:"created"`
	Closed  string `json:"closed,omitempty"`
}

// Entry is one repository's worth of upstream work, as the resume reads it.
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
		Number        int    `json:"number"`
		Title         string `json:"title"`
		HTMLURL       string `json:"html_url"`
		RepositoryURL string `json:"repository_url"`
		CreatedAt     string `json:"created_at"`
		ClosedAt      string `json:"closed_at"`
		PullRequest   struct {
			MergedAt string `json:"merged_at"`
		} `json:"pull_request"`
	} `json:"items"`
}

type options struct {
	user         string
	excludeOrgs  []string
	excludeRepos map[string]bool
	closedCounts map[string]bool
	out          string
	prsOut       string
	minMerged    int
	minOpen      int
}

func main() {
	var (
		user         = flag.String("user", "acouvreur", "GitHub user whose pull requests to collect")
		excludeOrgs  = flag.String("exclude-org", "reeveel,sablierapp", "comma-separated orgs to treat as own work, not upstream")
		excludeRepos = flag.String("exclude-repo", defaultExcludedRepos, "comma-separated owner/name repositories to omit")
		closedCounts = flag.String("closed-counts", defaultClosedCounts, "comma-separated owners or owner/name repositories where a closed PR counts as accepted")
		out          = flag.String("out", filepath.Join("data", "opensource.json"), "per-repository totals to write")
		prsOut       = flag.String("prs-out", filepath.Join("data", "pullrequests.json"), "full pull request list to write")
		minMerged    = flag.Int("min-merged", 1, "minimum accepted PRs for a repository to appear in the totals")
		minOpen      = flag.Int("min-open", 0, "if above zero, also include repositories with at least this many open PRs")
	)
	flag.Parse()

	opts := options{
		user:         *user,
		excludeOrgs:  splitList(*excludeOrgs),
		excludeRepos: toSet(splitList(*excludeRepos)),
		closedCounts: toSet(splitList(*closedCounts)),
		out:          *out,
		prsOut:       *prsOut,
		minMerged:    *minMerged,
		minOpen:      *minOpen,
	}

	if err := run(opts); err != nil {
		fmt.Fprintf(os.Stderr, "contributions: %v\n", err)
		os.Exit(1)
	}
}

func run(opts options) error {
	// A merged PR is also a closed one, so these two searches cover every state.
	closed, err := opts.fetch("is:closed")
	if err != nil {
		return fmt.Errorf("searching closed pull requests: %w", err)
	}
	open, err := opts.fetch("is:open")
	if err != nil {
		return fmt.Errorf("searching open pull requests: %w", err)
	}

	prs := append(closed, open...)
	if len(prs) == 0 {
		return fmt.Errorf("no pull requests found for %q, refusing to write empty files", opts.user)
	}

	// Newest first, and by repository then number within a day, so the file is
	// stable across runs and diffs stay readable.
	sort.Slice(prs, func(i, j int) bool {
		switch {
		case prs[i].Created != prs[j].Created:
			return prs[i].Created > prs[j].Created
		case prs[i].Repo != prs[j].Repo:
			return prs[i].Repo < prs[j].Repo
		default:
			return prs[i].Number > prs[j].Number
		}
	})

	if err := writeJSON(opts.prsOut, prs); err != nil {
		return err
	}

	entries := opts.summarise(prs)
	kept := carryOverProse(opts.out, entries)
	if err := writeJSON(opts.out, entries); err != nil {
		return err
	}

	report(opts, prs, entries, kept)
	return nil
}

// summarise rolls the pull requests up per repository for the resume.
func (o options) summarise(prs []PullRequest) []Entry {
	byRepo := map[string]*Entry{}
	for _, pr := range prs {
		entry, ok := byRepo[pr.Repo]
		if !ok {
			entry = &Entry{Name: pr.Repo}
			byRepo[pr.Repo] = entry
		}
		switch pr.State {
		case stateMerged:
			entry.Merged++
		case stateLanded:
			entry.Landed++
		case stateOpen:
			entry.Open++
		}
		// Declined PRs are deliberately not counted anywhere.
	}

	entries := make([]Entry, 0, len(byRepo))
	for _, entry := range byRepo {
		if o.includes(*entry) {
			entries = append(entries, *entry)
		}
	}

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
	return entries
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

// fetch pages through the search API for one state qualifier.
func (o options) fetch(state string) ([]PullRequest, error) {
	query := []string{
		"author:" + o.user,
		"type:pr",
		state,
		// Contributions to one's own repositories are projects, not upstream work.
		"-user:" + o.user,
	}
	for _, org := range o.excludeOrgs {
		query = append(query, "-org:"+org)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	token := githubToken()
	var prs []PullRequest

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
			repo := repoFromAPIURL(item.RepositoryURL)
			if repo == "" || o.excludeRepos[repo] {
				continue
			}
			prs = append(prs, PullRequest{
				Repo:    repo,
				Number:  item.Number,
				Title:   item.Title,
				URL:     item.HTMLURL,
				State:   o.classify(repo, state, item.PullRequest.MergedAt),
				Type:    typeOf(item.Title),
				Created: day(item.CreatedAt),
				Closed:  day(item.ClosedAt),
			})
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

	return prs, nil
}

func (o options) classify(repo, state, mergedAt string) string {
	if state == "is:open" {
		return stateOpen
	}
	if mergedAt != "" {
		return stateMerged
	}
	if o.countsClosed(repo) {
		return stateLanded
	}
	return stateClosed
}

// typeOf reads the conventional commit type off a pull request title.
func typeOf(title string) string {
	match := conventionalPrefix.FindStringSubmatch(title)
	if match == nil {
		return "other"
	}
	prefix := strings.ToLower(match[1])
	if !knownTypes[prefix] {
		return "other"
	}
	return prefix
}

// day trims an RFC 3339 timestamp down to its date.
func day(timestamp string) string {
	if len(timestamp) < 10 {
		return ""
	}
	return timestamp[:10]
}

func report(opts options, prs []PullRequest, entries []Entry, kept int) {
	counts := map[string]int{}
	for _, pr := range prs {
		counts[pr.State]++
	}
	fmt.Printf("wrote %s: %d pull requests (%d merged, %d landed, %d open, %d declined)\n",
		opts.prsOut, len(prs), counts[stateMerged], counts[stateLanded], counts[stateOpen], counts[stateClosed])

	fmt.Printf("wrote %s: %d repositories", opts.out, len(entries))
	if kept > 0 {
		fmt.Printf(", kept %d hand-written summaries", kept)
	}
	fmt.Println()
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

func writeJSON(path string, value any) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	encoded, err := json.MarshalIndent(value, "", "  ")
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
