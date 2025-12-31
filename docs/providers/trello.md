# Trello Provider

**Schemes:** `trello:`, `tr:`

**Capabilities:** `read`, `list`, `fetch_comments`, `comment`, `update_status`, `manage_labels`, `create_work_unit`, `download_attachment`, `snapshot`, `fetch_subtasks`

Integrates with Trello boards and cards for visual project management.

## Usage

```bash
# By card ID (24-character alphanumeric)
mehr start trello:507f1f77bcf86cd799439011
mehr plan tr:507f1f77bcf86cd799439011

# By short link (8-character)
mehr start trello:abc12XYZ
mehr start tr:abc12XYZ

# By card URL
mehr start trello:https://trello.com/c/abc12XYZ/card-name
mehr start tr:https://trello.com/c/abc12XYZ/card-name
```

## Configuration

```yaml
trello:
  api_key: "${TRELLO_API_KEY}"    # Trello API key
  token: "${TRELLO_TOKEN}"         # Trello token
  board: "default-board-id"        # Optional: default board for operations
```

## Token Resolution

1. `MEHR_TRELLO_API_KEY` / `MEHR_TRELLO_TOKEN` environment variables
2. `TRELLO_API_KEY` / `TRELLO_TOKEN` environment variables
3. Keys from `config.yaml`

## Authentication

Generate API credentials at: https://trello.com/app-key

1. Get your API key from the Trello developer page
2. Generate a token with the link provided on that page
3. Set both `api_key` and `token` in your configuration

## Reference Formats

| Format | Example |
|--------|---------|
| Scheme with card ID | `trello:507f1f77bcf86cd799439011` |
| Short scheme | `tr:507f1f77bcf86cd799439011` |
| Short link (8 chars) | `trello:abc12XYZ` |
| Card URL | `trello:https://trello.com/c/abc12XYZ/card-name` |

## Features

- **Card Fetching**: Retrieves card title, description, due date, labels, members, attachments, checklists
- **List Cards**: Browse cards from boards
- **Comment Support**: Fetch comments and add new ones
- **Status Updates**: Move cards between lists to change status
- **Label Management**: Add and remove labels on cards
- **Attachments**: Download file attachments
- **Checklists**: Fetch checklist items and their completion state

## Status Mapping

Status is determined by the card's list name (case-insensitive):

| Trello List | Provider Status |
|-------------|-----------------|
| To Do, Backlog, Todo | Open |
| In Progress, Doing, Started | In Progress |
| In Review, Review, Reviewing | Review |
| Done, Complete, Completed, Finished | Done |
| Archived, Cancelled, Canceled | Closed |

## Moving Cards (Status Change)

To change a card's status, the provider moves it to a list with a matching name:

```bash
# This will move the card to a list named "In Progress" (or similar)
mehr status set in_progress
```

The provider searches for lists with names matching the target status.
