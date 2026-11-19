---
name: lastab
description: Read the user's Lastab snippets (plain-text notes, commands, JSON, to-do lists they pasted or typed in the Lastab app). Use when the user refers to something they wrote, pasted or edited earlier ("the curl I had", "my release checklist", "what I noted this morning").
---

<!-- lastab-skill-version: 1 (Lastab offers "Reinstall" when the version it ships is higher) -->

# Lastab

Lastab keeps every snippet the user types or pastes, with titles made on the device. The `lastab` command reads them, read-only, as JSON. Read a snippet's full text only when the user asks for it.

## Commands

```
lastab search <query> [--since 1d] [--until 2026-11-18] [--kind text|todo] [--limit 20] --client claude-code
lastab list [--open] [--since 2h] [--kind text|todo] [--limit 20] --client claude-code
lastab read <id or unique prefix> --client claude-code
lastab status
```

- `search` matches any part of a title or text, case-insensitively, Chinese included; results come with a short masked `preview`.
- `--since` takes `30m`, `2h`, `1d`, `1w` or a date; `--open` keeps snippets the user has open as tabs.
- `read` returns `body` exactly as stored; secrets are masked (`"redacted": true`) unless the user allowed raw output. To-do lists add `items` and `progress`.

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | OK |
| 2 | Bad arguments |
| 3 | The user turned agent access off in Lastab's settings: tell them, don't retry |
| 4 | No snippet with that ID, or the prefix matches several |
| 5 | No Lastab database, or it is newer than this CLI |

Errors print `{"error": {"kind": "...", "message": "..."}}`.
