package codegraph

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Graph stores code symbols and relationships in SQLite.
type Graph struct {
	db      *sql.DB
	rootDir string
}

// Symbol represents a code symbol (function, type, interface, method).
type Symbol struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Kind    string `json:"kind"` // "function", "type", "interface", "method", "const", "var"
	File    string `json:"file"`
	Line    int    `json:"line"`
	Package string `json:"package"`
}

// Edge represents a relationship between symbols.
type Edge struct {
	FromID   int64  `json:"from_id"`
	ToID     int64  `json:"to_id"`
	Relation string `json:"relation"` // "calls", "implements", "embeds", "references"
}

const schema = `
CREATE TABLE IF NOT EXISTS symbols (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL,
    kind TEXT NOT NULL,
    file TEXT NOT NULL,
    line INTEGER NOT NULL,
    package TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_symbols_name ON symbols(name);
CREATE INDEX IF NOT EXISTS idx_symbols_file ON symbols(file);
CREATE INDEX IF NOT EXISTS idx_symbols_package ON symbols(package);

CREATE TABLE IF NOT EXISTS edges (
    from_id INTEGER NOT NULL REFERENCES symbols(id),
    to_id INTEGER NOT NULL REFERENCES symbols(id),
    relation TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id);
CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id);
`

// New opens or creates a SQLite-backed code graph at dbPath.
func New(ctx context.Context, dbPath string) (*Graph, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Enable WAL mode for better concurrent read performance.
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	// Enable foreign key constraints for referential integrity.
	if _, err := db.ExecContext(ctx, "PRAGMA foreign_keys=ON"); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("enable foreign keys: %w", err)
	}

	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()

		return nil, fmt.Errorf("create schema: %w", err)
	}

	// rootDir is left empty here; it is set by IndexDirectory() to enable
	// relative path storage. Calling IndexFile() without IndexDirectory()
	// stores absolute paths, which is valid but less portable.
	return &Graph{db: db}, nil
}

// Close closes the underlying database connection.
func (g *Graph) Close() error {
	if g.db == nil {
		return nil
	}

	return g.db.Close()
}

// IndexFile parses a single Go file and indexes its symbols and edges.
func (g *Graph) IndexFile(ctx context.Context, path string) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("abs path: %w", err)
	}

	// Compute relative path for storage if rootDir is set.
	storePath := absPath
	if g.rootDir != "" {
		if rel, relErr := filepath.Rel(g.rootDir, absPath); relErr == nil {
			storePath = rel
		}
	}

	symbols, edges, err := parseGoFile(absPath)
	if err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	tx, err := g.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck // rollback after commit is harmless

	// Remove old symbols and edges for this file.
	if _, err := tx.ExecContext(ctx, "DELETE FROM edges WHERE from_id IN (SELECT id FROM symbols WHERE file = ?) OR to_id IN (SELECT id FROM symbols WHERE file = ?)", storePath, storePath); err != nil {
		return fmt.Errorf("delete old edges: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM symbols WHERE file = ?", storePath); err != nil {
		return fmt.Errorf("delete old symbols: %w", err)
	}

	// Insert symbols and build a name->id map for edge resolution.
	nameToID := make(map[string]int64)
	for i := range symbols {
		symbols[i].File = storePath
		res, err := tx.ExecContext(ctx,
			"INSERT INTO symbols (name, kind, file, line, package) VALUES (?, ?, ?, ?, ?)",
			symbols[i].Name, symbols[i].Kind, symbols[i].File, symbols[i].Line, symbols[i].Package,
		)
		if err != nil {
			return fmt.Errorf("insert symbol %s: %w", symbols[i].Name, err)
		}
		id, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("get last insert id for %s: %w", symbols[i].Name, err)
		}
		symbols[i].ID = id
		nameToID[symbols[i].Name] = id
	}

	// Insert edges — resolve names to IDs within this file, or look up globally.
	for _, e := range edges {
		fromID, ok := nameToID[e.FromName]
		if !ok {
			continue
		}

		toID, ok := nameToID[e.ToName]
		if !ok {
			// Try to find the target symbol in the database (from other files).
			row := tx.QueryRowContext(ctx, "SELECT id FROM symbols WHERE name = ? LIMIT 1", e.ToName)
			if err := row.Scan(&toID); err != nil {
				continue // Skip unresolved edges.
			}
		}

		if _, err := tx.ExecContext(ctx,
			"INSERT INTO edges (from_id, to_id, relation) VALUES (?, ?, ?)",
			fromID, toID, e.Relation,
		); err != nil {
			return fmt.Errorf("insert edge: %w", err)
		}
	}

	return tx.Commit()
}

// IndexDirectory walks dir and indexes all .go files (excluding vendor, testdata).
func (g *Graph) IndexDirectory(ctx context.Context, dir string) error {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("abs dir: %w", err)
	}
	g.rootDir = absDir

	var indexed int
	walkErr := filepath.WalkDir(absDir, func(path string, d os.DirEntry, walkDirErr error) error {
		if walkDirErr != nil {
			slog.Debug("skipping inaccessible entry", "path", path, "error", walkDirErr)

			return filepath.SkipDir
		}

		name := d.Name()

		// Skip common non-source directories.
		if d.IsDir() {
			switch name {
			case "vendor", "testdata", "node_modules", ".git":
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(name, ".go") {
			return nil
		}

		if err := g.IndexFile(ctx, path); err != nil {
			slog.Debug("skipping file", "path", path, "error", err)

			return nil
		}
		indexed++

		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk directory: %w", walkErr)
	}

	slog.Info("code graph indexed", "dir", dir, "files", indexed)

	return nil
}

// QueryCallersOf finds symbols that call the given function name.
func (g *Graph) QueryCallersOf(ctx context.Context, name string) ([]Symbol, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT s.id, s.name, s.kind, s.file, s.line, s.package
		FROM symbols s
		JOIN edges e ON e.from_id = s.id
		JOIN symbols target ON e.to_id = target.id
		WHERE target.name = ? AND e.relation = 'calls'
	`, name)
	if err != nil {
		return nil, fmt.Errorf("query callers: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// QueryDependenciesOf finds packages imported by the given package.
func (g *Graph) QueryDependenciesOf(ctx context.Context, pkg string) ([]string, error) {
	rows, err := g.db.QueryContext(ctx, `
		SELECT DISTINCT target.package
		FROM symbols s
		JOIN edges e ON e.from_id = s.id
		JOIN symbols target ON e.to_id = target.id
		WHERE s.package = ? AND e.relation = 'imports'
	`, pkg)
	if err != nil {
		return nil, fmt.Errorf("query dependencies: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var deps []string
	for rows.Next() {
		var dep string
		if err := rows.Scan(&dep); err != nil {
			return nil, fmt.Errorf("scan dependency: %w", err)
		}
		deps = append(deps, dep)
	}

	return deps, rows.Err()
}

// QuerySymbol finds symbols matching the given name (exact match).
func (g *Graph) QuerySymbol(ctx context.Context, name string) ([]Symbol, error) {
	rows, err := g.db.QueryContext(ctx,
		"SELECT id, name, kind, file, line, package FROM symbols WHERE name = ?",
		name,
	)
	if err != nil {
		return nil, fmt.Errorf("query symbol: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// QuerySymbolPattern finds symbols matching a LIKE pattern (use % for wildcards).
func (g *Graph) QuerySymbolPattern(ctx context.Context, pattern string) ([]Symbol, error) {
	rows, err := g.db.QueryContext(ctx,
		"SELECT id, name, kind, file, line, package FROM symbols WHERE name LIKE ?",
		pattern,
	)
	if err != nil {
		return nil, fmt.Errorf("query symbol pattern: %w", err)
	}
	defer func() { _ = rows.Close() }()

	return scanSymbols(rows)
}

// Stats returns counts of symbols and edges by kind/relation.
func (g *Graph) Stats(ctx context.Context) map[string]int {
	stats := make(map[string]int)

	// Total symbols.
	row := g.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM symbols")
	var count int
	if err := row.Scan(&count); err == nil {
		stats["symbols"] = count
	}

	// Symbols by kind.
	if err := g.collectGroupCounts(ctx, "SELECT kind, COUNT(*) FROM symbols GROUP BY kind", "symbols_", stats); err != nil {
		slog.Debug("stats: symbols by kind", "error", err)
	}

	// Total edges.
	row = g.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM edges")
	if err := row.Scan(&count); err == nil {
		stats["edges"] = count
	}

	// Edges by relation.
	if err := g.collectGroupCounts(ctx, "SELECT relation, COUNT(*) FROM edges GROUP BY relation", "edges_", stats); err != nil {
		slog.Debug("stats: edges by relation", "error", err)
	}

	// File count.
	row = g.db.QueryRowContext(ctx, "SELECT COUNT(DISTINCT file) FROM symbols")
	if err := row.Scan(&count); err == nil {
		stats["files"] = count
	}

	return stats
}

// collectGroupCounts runs a "SELECT label, COUNT(*)" query and stores results in stats with the given prefix.
func (g *Graph) collectGroupCounts(ctx context.Context, query, prefix string, stats map[string]int) error {
	rows, err := g.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var label string
		var c int
		if err := rows.Scan(&label, &c); err == nil {
			stats[prefix+label] = c
		}
	}

	return rows.Err()
}

func scanSymbols(rows *sql.Rows) ([]Symbol, error) {
	var symbols []Symbol
	for rows.Next() {
		var s Symbol
		if err := rows.Scan(&s.ID, &s.Name, &s.Kind, &s.File, &s.Line, &s.Package); err != nil {
			return nil, fmt.Errorf("scan symbol: %w", err)
		}
		symbols = append(symbols, s)
	}

	return symbols, rows.Err()
}
