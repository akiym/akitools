---
name: docidx
description: Search locally indexed library documentation (SQLite FTS5/BM25) via the `docidx` CLI. Use when the user asks how to do something in a library/framework whose docs are indexed into an index.db (API/class/method lookups, tutorials, guides, FAQs), or asks to search/read indexed docs. Flow - search for candidates, then cat the promising chunk ids.
allowed-tools: Bash(docidx search:*), Bash(docidx cat:*)
---

# docidx

Fast BM25 search over locally indexed documentation. `docidx build` splits
Markdown/HTML files on their heading structure (one chunk per section or API
method) into an SQLite FTS5 index; `search` and `cat` read it.

## Workflow

```bash
docidx search --db path/to/index.db spawn enemy   # 1. get candidates (TSV)
docidx cat --db path/to/index.db 42 43            # 2. read the promising chunks
docidx cat --db path/to/index.db --path 'tutorials/instancing.md#anchor'  # 3. whole page
```

`--db` defaults to `./index.db`. Multi-word queries are passed as separate
args, no quoting needed. Favor recall: cat several small candidates at once
rather than only the top hit. `cat --path` accepts the `path#anchor` column
from search output as-is (the anchor is ignored) and prints the whole page.

## search output (TSV)

Columns: `id`, `score`, `kind`, `body bytes`, `path#anchor`, `title`

- `score`: higher is better. Rows matching ALL query terms come first; rows
  after the `# or-fallback: ...` marker line match only some terms. Scores
  restart at the marker — compare scores only within each group.
- `kind`: e.g. `api` / `class` / `method` / `tutorial` / `guide` / `faq` /
  `doc`; which ones appear depends on the indexed docs.
- `body bytes`: check before cat. Large chunks (over ~10KB) are usually
  aggregate listings (member tables, section indexes) — prefer small,
  specific chunks first. But in API-reference indexes the aggregate table is
  sometimes the only place a signature appears, so fall back to it when the
  small chunks don't have the answer.

## Query tips

- Exact identifiers beat prose: class names, method names, error codes.
  CamelCase is one token — search `AnimationPlayer`, not `animation player`.
- `no results` (exit 0) is a normal miss, not a tool failure. Recover by
  searching a coarser term (the class name alone), a synonym, or cat the
  class's aggregate chunk (e.g. its `#header-pub-methods` hit) and read the
  listing directly.
- If a natural-language query returns scattered results, pick an identifier
  from any promising hit and search again with it.
- An `aliases.json` next to the index expands query terms automatically
  (e.g. `{"spawn": ["instantiate", "PackedScene"]}`).

## Building an index

`docidx build` deletes and recreates the `--db` file, so it is intentionally
not pre-approved. Run it only when the user asks to index docs:

```bash
docidx build docs/ --db index.db
# doxygen output — skip source listings and member tables:
docidx build html/ --db index.db --exclude '*_source.html' --exclude '*-members.html'
```

`--exclude` takes gitignore-syntax patterns relative to the docs dir,
repeatable.
