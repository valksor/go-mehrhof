# Directory Provider

> **⚠️ Experimental**: This provider is not fully tested beyond unit tests. Edge cases may exist. Manual validation recommended before production use.

**Schemes:** `dir:`

**Capabilities:** `read`, `list`

Reads tasks from markdown files in a local directory. Can list all available task files.

## Usage

```bash
mehr start dir:./tasks
mehr plan dir:./docs
```

## Listing Files

The directory provider can enumerate all markdown files in a directory, allowing you to browse available tasks before starting one.
