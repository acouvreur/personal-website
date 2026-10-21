+++
title = 'Back to Basics: direnv'
date = 2026-10-20T10:00:00-04:00
draft = false
tags = ['direnv', 'vault', 'devex']
description = 'Every post is about the next AI tool. This one goes back to basics: a small, old tool that made onboarding at work a non-event.'
+++

Most of what I read these days is about something new. A new model, a new agent, a
new way to not write code. Fine. This post goes back to basics: a small tool that has
been around for years and quietly removed a whole category of onboarding problems at
work.

The tool is [direnv](https://direnv.net).

## The problem

Every project needs an environment to run. Default config, like which Vault to talk to
or the log level. And secrets, like a database password. Before, everyone kept a local
`.env` with all of that copied in by hand. It worked on day one. Then a password was
rotated or a default changed, and your copy was quietly out of date. And since that
file was full of real values, it was always one `.gitignore` slip away from a commit.

## What direnv does

direnv loads environment variables when you `cd` into a directory and unloads them
when you leave. The variables come from a file called `.envrc` at the root of the
project. In other words, the environment follows the directory you're in.

What makes it work is the shell hook. One line in your shell config:

```sh
# ~/.zshrc
eval "$(direnv hook zsh)"
```

From then on, direnv runs before every prompt. Enter a directory with an `.envrc` and
it loads it. Leave it and your environment goes back to how it was. Edit the `.envrc`
and it reloads. You never call it yourself.

> [!NOTE]
> direnv won't run a file it hasn't seen before. The first time, and after every
> change, it asks you to run `direnv allow`. An `.envrc` is a shell script, so that's
> the right default.

## Our `.envrc`

Every repo has one, committed next to the code. Plain exports for default config, a
Vault login, and the secrets pulled from Vault:

```bash
export VAULT_ADDR=https://vault.example.com
export LOG_LEVEL=debug

vault token lookup >/dev/null 2>&1 || vault login -method=oidc >/dev/null
export DB_PASSWORD=$(vault kv get -field=password secret/myapp/dev/db)
```

The `vault token lookup` line checks whether you already have a valid token. If you
don't, `vault login -method=oidc` opens the browser, you sign in with the same account
as everything else, and the token is cached. Most days that line does nothing.

So what changed?

- **Secrets are always in sync.** They come from Vault every time you enter the
  directory. When a password is rotated, nobody has to tell anyone.
- **No stale local config.** Defaults are reviewed and merged like any other change.
  You pull, direnv asks you to allow the new version, and you're up to date.
- **Nothing secret to commit by mistake.** The `.envrc` is meant to be committed, and
  it holds paths into Vault, not values.
- **The app knows nothing about Vault.** It reads environment variables and that's it.
  No Vault client, no credentials of its own, whatever the language.
- **Onboarding is a clone.** Install direnv and the Vault CLI, add the hook, clone the
  repo, run `direnv allow`.

## A word of caution

There's one catch, and it brings us back to the intro. The secrets live in your shell,
not in the app. Every process started from that terminal inherits them: a script, an
npm `postinstall`, or an AI agent working in the repo.

Those agents are really into our systems now. They read files, run commands, and see
`DB_PASSWORD` like everything else. In other words, your convenience is also a
security risk. If your AI agent runs in a sandbox, there's no issue. I'm sure that's
not the case for everyone yet.

## It just works

Once it's adopted, direnv becomes one of those tools that is so stable and so obvious
that you forget about it. Nobody questions it anymore. You switch to a repo and you
expect your context to already be loaded.
