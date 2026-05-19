# Semantic Memory

kvelmo includes a semantic memory system that helps agents understand your codebase better.

## Overview

The memory system:

- Indexes your codebase
- Creates embeddings for code and documentation
- Enables semantic search
- Provides context to agents

## Enabling Memory

Memory is enabled by default. Configure in settings:

```json
{
  "memory": {
    "enabled": true,
    "embedding_model": "tfidf"
  }
}
```

## Embedding Models

| Model       | Description            | Requirements |
| ----------- | ---------------------- | ------------ |
| `tfidf`     | TF-IDF based (default) | None         |
| `cybertron` | Neural embeddings      | Go-based     |

### TF-IDF

Fast, lightweight, no external dependencies. Good for most use cases.

### Cybertron

Higher quality embeddings using neural models. Requires more resources.

## Indexing

kvelmo indexes your codebase automatically. Manual indexing:

```bash
# Index current project
kvelmo memory index

# Force re-index
kvelmo memory index --force
```

## Searching

Search the memory:

```bash
# Semantic search
kvelmo memory search "authentication logic"

# Search with limit
kvelmo memory search "database queries" --limit 10
```

## Memory in Web UI

Access memory in the Web UI:

1. Click **Memory** in the sidebar
2. Enter a search query
3. Browse results

## How Agents Use Memory

During planning and implementation:

1. Agent formulates queries based on the task
2. Memory returns relevant code snippets
3. Agent uses context to make better decisions

## Storage

Memory is stored in `.kvelmo/memory/`:

```
.kvelmo/memory/
├── index.json       # Metadata
├── embeddings.bin   # Vector store
└── chunks/          # Indexed chunks
```

## Performance

For large codebases:

- Initial indexing may take time
- Subsequent updates are incremental
- Memory is project-local

## Clearing Memory

```bash
# Clear all memory
kvelmo memory clear

# Re-index
kvelmo memory index
```

## Configuration Options

```json
{
  "memory": {
    "enabled": true,
    "embedding_model": "tfidf",
    "chunk_size": 1000,
    "max_results": 20
  }
}
```

| Option            | Description          | Default |
| ----------------- | -------------------- | ------- |
| `enabled`         | Enable memory        | true    |
| `embedding_model` | Model to use         | tfidf   |
| `chunk_size`      | Characters per chunk | 1000    |
| `max_results`     | Max search results   | 20      |
