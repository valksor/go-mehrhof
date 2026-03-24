# Code Graph

The Code Graph panel lets you index and explore code symbols in your project.

## Opening

Click the database icon in the project view toolbar to open the Code Graph panel.

## Features

### Index Project

Click **Index** to parse all Go source files and build the symbol database. This creates a SQLite database at `.kvelmo/codegraph.db` containing functions, types, interfaces, methods, constants, and variables along with their relationships.

### Statistics

Click **Stats** to see how many files, symbols, and edges are indexed.

### Search Symbols

Type a symbol name and press Enter or click the search button. Results show each symbol's kind, name, file location, and package.

Enable the **Pattern** checkbox to use SQL LIKE wildcards (`%` matches any characters).

## Related

- [kvelmo codegraph](/cli/codegraph.md) — CLI reference
