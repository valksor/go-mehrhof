# Browser Automation

kvelmo includes browser automation capabilities for tasks that require web interaction.

## Overview

Browser automation allows agents to:

- Navigate web pages
- Take screenshots
- Interact with UI elements
- Test web applications

## Starting Browser

Start a browser session:

```bash
kvelmo browser start
```

Or from the Web UI: Click **Browser** in the sidebar.

## Browser Commands

### Navigate

```bash
kvelmo browser navigate https://example.com
```

### Screenshot

```bash
kvelmo browser screenshot
```

Screenshots are saved to `.kvelmo/screenshots/`.

### Execute Script

```bash
kvelmo browser eval "document.title"
```

## Web UI Browser Panel

The Browser panel in the Web UI shows:

- Current URL
- Page screenshot
- Console output
- Network requests

## Configuration

```json
{
  "browser": {
    "headless": true,
    "viewport": {
      "width": 1280,
      "height": 720
    }
  }
}
```

| Option            | Description     | Default |
| ----------------- | --------------- | ------- |
| `headless`        | Run without GUI | true    |
| `viewport.width`  | Browser width   | 1280    |
| `viewport.height` | Browser height  | 720     |

## Use Cases

### Testing Web Apps

```bash
# Start browser
kvelmo browser start

# Navigate to app
kvelmo browser navigate http://localhost:3000

# Take screenshot
kvelmo browser screenshot --name "home-page"
```

### Debugging UI

Use browser automation during implementation to verify UI changes.

### Documentation

Capture screenshots for documentation automatically.

## Agent Integration

Agents can use browser tools during implementation:

1. Agent decides to verify UI
2. Navigates to the relevant page
3. Takes screenshot for verification
4. Continues implementation

## Limitations

- Requires a browser to be installed
- Headless mode recommended for servers
- Some sites may block automation
