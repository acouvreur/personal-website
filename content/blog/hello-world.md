+++
title = 'This Site Runs on Hugo Now'
date = 2026-07-25T10:00:00-04:00
draft = false
tags = ['meta', 'hugo']
description = 'Moved off resume-cli and onto Hugo, so the resume and a blog can live in the same place.'
+++

For a few years this site was one file. A `resume.json`, `resume-cli`, the elegant
theme, and out came a single `index.html` that GitHub Pages served. It worked fine.
The problem was that it could only ever be a resume.

So I moved it to Hugo. The resume still comes from the same `resume.json`, it just
lives in `data/` now and a Hugo template renders it instead of a Node theme. I edit
one JSON file and the [resume page](/resume/) updates. The schema didn't change, so
it stays portable if I ever want to take it somewhere else.

One thing I did change: the open source contributions on that page are generated.
There's a small Go program that asks the GitHub API which pull requests I've had
merged where, and writes the result to a data file. I got tired of numbers that were
wrong three months after I typed them.

The actual reason for all this is that I wanted somewhere to write. Infrastructure,
Go, containers, the occasional detour into other people's repos. First real post
soon.
