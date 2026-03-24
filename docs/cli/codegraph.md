# kvelmo codegraph

Code symbol graph — index, search, and explore code symbols and their relationships.

## Usage

```bash
kvelmo codegraph <subcommand>
```

## Subcommands

| Command              | Description                          |
|----------------------|--------------------------------------|
| `stats`              | Show code graph statistics           |
| `index [path]`       | Index code symbols in a directory    |
| `search <name>`      | Find symbol definitions              |
| `callers <function>` | Find callers of a function           |
| `deps <package>`     | Find package dependencies            |

## Flags

| Flag        | Description                                |
|-------------|--------------------------------------------|
| `--json`    | Output raw JSON response                   |
| `--pattern` | Use LIKE pattern matching (search only)    |

## Examples

```bash
# Index the current project
kvelmo codegraph index

# Index a specific directory
kvelmo codegraph index ./pkg/conductor

# Show indexing statistics
kvelmo codegraph stats

# Find a symbol by exact name
kvelmo codegraph search HandleStart

# Find symbols by pattern (% = wildcard)
kvelmo codegraph search --pattern "Handle%"

# Find callers of a function
kvelmo codegraph callers NewConductor

# Find package dependencies
kvelmo codegraph deps conductor
```

## How It Works

The code graph parses Go source files and stores symbols (functions, types, interfaces, methods, constants, variables) and their relationships (calls, implements, embeds, references) in a SQLite database at `.kvelmo/codegraph.db`.

Indexing skips `vendor/`, `testdata/`, `node_modules/`, and `.git/` directories.

## Related

- [Code Graph Panel](/web-ui/codegraph.md) — Web UI interface
