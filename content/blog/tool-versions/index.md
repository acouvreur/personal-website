+++
title = '.tool-versions: Same Versions in Dev and CI'
series = ['Back to Basics']
date = 2026-10-21T00:00:00-04:00
draft = false
tags = ['mise', 'github-actions', 'devex']
description = 'Pin runtimes and tools once in .tool-versions, and let both your machine and GitHub Actions read it. Dev and CI stop drifting apart.'
+++

Work on a few projects and you start collecting version managers.
[nvm](https://github.com/nvm-sh/nvm) for Node, [SDKMAN](https://sdkman.io) for Java,
[pyenv](https://github.com/pyenv/pyenv) for Python, [tfenv](https://github.com/tfutils/tfenv)
for Terraform. Each one has its own install, its own setup in your shell, and its own
file: `.nvmrc`, `.sdkmanrc`, `.python-version`, `.terraform-version`.

## One manager per language doesn't scale

They do their job. Node has had `.nvmrc` for years, so pinning a version isn't the hard
part. Keeping all those managers around is. Every new project can mean one more on
every laptop, and CI has to be told the same versions again, in its own way.

And not every tool has a manager. A Go project also needs `golangci-lint` and
`goreleaser`, and those need a version too. Go 1.24 added
[`tool` directives](https://go.dev/doc/go1.24#tools) to `go.mod`, which look like the
answer, but they build the tool from source, mixed in with your project's dependencies.
golangci-lint's own docs say it plainly: installing it with `go install`, the tools
pattern or `tool` directives
["aren't guaranteed to work"](https://golangci-lint.run/docs/welcome/install/local/#install-from-sources),
and they recommend the binary.

So the version ends up hardcoded in the workflow YAML while your machine runs whatever
you installed last year. Same drift, one tool at a time.

## One file for everything

`.tool-versions` comes from [asdf](https://asdf-vm.com/manage/configuration.html#tool-versions),
and [mise](https://mise.jdx.dev) reads it too. One line per tool:

```
golang 1.25.3
nodejs 22.11.0
terraform 1.13.4
golangci-lint 2.6.0
goreleaser 2.12.0
```

With mise activated in your shell, you `cd` into the repo and these exact versions
are on your `PATH`. `mise install` fetches whatever is missing.

If you were counting on `.nvmrc`, note that mise stopped reading the per-language
files by default in [2025.10.0](https://github.com/jdx/mise/releases/tag/v2025.10.0)
([#6501](https://github.com/jdx/mise/pull/6501)). You can turn them back on, tool by tool. One more reason
to keep everything in one file.

## CI reads the same file

This is the part that changed over the last few years: adoption. It started with the
official setup actions, and it now reaches tools like golangci-lint and goreleaser.
One action after another adopted `.tool-versions`:

- `actions/setup-node` with `node-version-file`, since
  [v3.4.0](https://github.com/actions/setup-node/releases/tag/v3.4.0) in 2022
  ([#373](https://github.com/actions/setup-node/pull/373))
- `actions/setup-java` with `java-version-file`, since
  [v4.2.0](https://github.com/actions/setup-java/releases/tag/v4.2.0) in 2024
  ([#606](https://github.com/actions/setup-java/pull/606))
- `actions/setup-python` with `python-version-file`, since
  [v5.5.0](https://github.com/actions/setup-python/releases/tag/v5.5.0) in 2025
  ([#1043](https://github.com/actions/setup-python/pull/1043))
- `actions/setup-go` with `go-version-file`, since
  [v6.1.0](https://github.com/actions/setup-go/releases/tag/v6.1.0) in 2025
  ([#673](https://github.com/actions/setup-go/pull/673))
- `golangci/golangci-lint-action` with `version-file`, since
  [v9.2.0](https://github.com/golangci/golangci-lint-action/releases/tag/v9.2.0) in 2025
  ([#1320](https://github.com/golangci/golangci-lint-action/pull/1320))
- `goreleaser/goreleaser-action` with `version-file`, since
  [v7.2.0](https://github.com/goreleaser/goreleaser-action/releases/tag/v7.2.0) in 2026
  ([#556](https://github.com/goreleaser/goreleaser-action/pull/556))

So a Go and Node workflow looks like this:

```yaml
steps:
  - uses: actions/checkout@v7
  - uses: actions/setup-go@v7
    with:
      go-version-file: .tool-versions
  - uses: actions/setup-node@v7
    with:
      node-version-file: .tool-versions
  - uses: golangci/golangci-lint-action@v9
    with:
      version-file: .tool-versions
  - uses: goreleaser/goreleaser-action@v7
    with:
      version-file: .tool-versions
      args: release --clean
```

No version in the YAML. Bump a line in `.tool-versions` and your machine and CI move
together. In other words, there's no second place to forget.

## Why `.tool-versions` and not `mise.toml`?

mise has its own file, `mise.toml`, and it does more: environment variables, tasks.
[This site uses it](https://github.com/acouvreur/personal-website/blob/main/mise.toml). So why not use it everywhere?

Compatibility with the actions. `setup-node` has started reading `mise.toml` in
[v7.1.0](https://github.com/actions/setup-node/releases/tag/v7.1.0)
([#1421](https://github.com/actions/setup-node/pull/1421)), but it's the exception. For the rest, `.tool-versions` is the format they all understand.

> [!TIP] Keep the file boring
> The actions read `.tool-versions` with their own simple parsers, not with mise.
> `setup-go` [only looks for a `golang` line](https://github.com/actions/setup-go/blob/90ad2b35f69faf97585ad74d28fa006d2739b7af/src/installer.ts#L668),
> not `go`. `setup-python`
> [skips a line with a trailing comment](https://github.com/actions/setup-python/blob/ec52d0af729a7c50ced4261d17a07916f43f5b8e/src/utils.ts#L337). And mise-only syntax like `latest` or `aqua:` backends
> means nothing to them. One tool, one exact version per line, and everything reads it.

## When the action doesn't read it

Not every action is there yet. `hashicorp/setup-terraform`
[only takes a `terraform_version` input](https://github.com/hashicorp/setup-terraform/blob/b75000b53e9ce4de2b4712237bbdb711bdc92aba/action.yml#L12).
For those, [`jdx/mise-action`](https://github.com/jdx/mise-action) reads `.tool-versions` and
installs everything listed in it:

```yaml
  - uses: jdx/mise-action@v5
```

It could replace all the setup actions above too, in a single step. But it's a
tradeoff, because the setup actions do more than install a binary:

- **Problem matchers.** `setup-go`
  [registers one](https://github.com/actions/setup-go/blob/90ad2b35f69faf97585ad74d28fa006d2739b7af/matchers.json)
  so compiler errors show up as annotations on the pull request. `setup-node`
  [does the same for `tsc` and ESLint](https://github.com/actions/setup-node/blob/949feb2413d6458794dcd2491c4babbbce0c15c1/src/main.ts#L91-L101).
- **Dependency caching.** `setup-go`
  [caches modules and build outputs by default](https://github.com/actions/setup-go#caching-dependency-files-and-build-outputs),
  and `setup-node` [caches npm, yarn or pnpm data](https://github.com/actions/setup-node#caching-global-packages-data).
  `mise-action` caches the tools it installs, not your dependencies.
- **Language setup.** `setup-node`
  [writes the `.npmrc`](https://github.com/actions/setup-node/blob/main/docs/advanced-usage.md#publish-to-npmjs-and-gpr-with-npm)
  for a private registry or for publishing.

With `mise-action`, each of those is an extra step you write yourself. For Terraform
there's nothing extra to set up, so it's the obvious choice. For Go and Node, the setup
actions save those steps. Having both options comes in handy.

{{< diagram src="readers.svg" caption="One file, every reader. Terraform's own action doesn't read it yet, so `mise-action` installs it (dashed)." >}}

## Where it stands

Adoption keeps going. Every few months another action adopts it, and goreleaser was
the latest, in
[April](https://github.com/goreleaser/goreleaser-action/releases/tag/v7.2.0).
Terraform isn't there yet, so for now that's the one line where `mise-action` does the
job.
