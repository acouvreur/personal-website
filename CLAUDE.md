# CLAUDE.md

## Writing blog posts

Posts live in `content/blog/`, are written in English, and start with `draft = true`.
Alexis edits every draft before it goes out, so write plainly and keep it easy to
rework. The voice below comes from his own writing.

- **Problem first.** Describe how things were and what was wrong before the solution.
- **Say when you backed out of something, and why.** Prefer removing a layer over
  adding one; explain the real problem that made it unnecessary.
- **Be honest about limits.** No selling. "On the other hand", "for now", "I doubt the
  effort is worth it" are all in character.
- **Concrete numbers** over adjectives, but only real ones. Ask rather than invent.
- **Everyday analogies** for technical things (a config file "like a shopping list").
- **Explain a term right after introducing it** ("In other words, ...").
- **Questions as transitions** ("So what changed?").
- **End on the current state or what comes next**, not a moral or a slogan.
- **Short, first-person sentences.** Starting a sentence with "And" or "But" is fine.
  No em dashes.
- Recurring values: simple, generic, extensible, one command to deploy, config as
  code next to the code it configures.

For emphasis, posts use GitHub-style alerts (`> [!NOTE]`, `[!TIP]`, `[!IMPORTANT]`,
`[!WARNING]`, `[!CAUTION]`, optional title after the marker), rendered by
`layouts/_markup/render-blockquote.html`. Use one or two per post at most.

Don't invent details about Alexis's company setup (tools, numbers, past workflow).
Ask, or leave it out.
