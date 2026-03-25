# Changelog

Generate release changelogs between two git refs directly from the web UI.

## Opening

Open the Changelog panel from the sidebar. It opens as a modal overlay.

## Generating a Changelog

1. Enter a **Source** ref (tag, branch, or commit SHA) — the starting point
2. Enter a **Target** ref — the ending point
3. Optionally enable **Full descriptions** to include commit body text
4. Click **Generate**

The panel calls `changelog.generate` and renders the resulting Markdown.

## Options

| Option | Description |
|--------|-------------|
| Source | Starting git ref (e.g., `v1.0.0`, `main~10`, a commit SHA) |
| Target | Ending git ref (e.g., `HEAD`, `v2.0.0`) |
| Full | Include commit body text alongside summary lines |

## Copying

Click **Copy** to copy the generated Markdown to the clipboard for pasting into release notes, PRs, or documentation.

## Chat Command

You can also generate changelogs from the chat input:

```
/changelog v1.0..v2.0
/changelog full v1.0..v2.0
```

## Related

- [kvelmo changelog](/cli/changelog.md) — CLI equivalent
- [Export](/web-ui/export.md) — Export task data in other formats
