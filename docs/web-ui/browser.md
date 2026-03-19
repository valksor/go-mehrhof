# Browser

Control a Playwright browser instance for interactive testing of the target project.

## Installation

If the browser runtime is not installed, the panel shows an **Install Browser Runtime** button. Click it to download and set up Playwright. The status bar at the bottom shows the active browser engine (chromium, firefox, or webkit) and whether it runs headless or headed.

## Navigation

The top bar provides standard browser controls:

- **Back / Forward / Reload** buttons
- **URL bar** — Enter a URL and press Enter or click **Go**

The current page title and URL are shown below the address bar.

## Tabs

The panel is organized into six tabs:

### Navigate

- **Wait for Element** — Enter a CSS selector to wait until it appears
- **Scroll Page** — Choose direction (up, down, left, right) and pixel amount
- **Evaluate JavaScript** — Run arbitrary JS in the page context and see the result

### Interact

- **Click Element** — Click an element by CSS selector
- **Hover / Focus** — Hover over or focus an element
- **Press Key** — Send a keypress (Enter, Escape, Tab, arrow keys, or combos like `Control+a`)
- **Handle Dialog** — Accept or dismiss alert/confirm/prompt dialogs

### Forms

- **Type Text** — Type into an element character by character
- **Fill Input** — Clear and set a value instantly
- **Select Option** — Choose a `<select>` option by value or label
- **Upload File** — Upload a file to a file input by absolute path

### Capture

- **Screenshot** — Capture the visible viewport or the full page
- **Accessibility Snapshot** — Get the page structure as an accessibility tree
- **Generate PDF** — Export the page as A4 portrait or landscape PDF

### Console

View captured browser console messages (log, info, warning, error). Click **Refresh** to update.

### Network

View captured network requests in a table showing method, URL, status code, and resource type. Click **Refresh** to update.
