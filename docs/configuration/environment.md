# Environment Variables

kvelmo reads sensitive configuration (tokens, secrets) from `.env` files only. OS-level environment variables are not used.

## dotenv Files

kvelmo loads `.env` files from two locations:

1. **Global**: `~/.valksor/kvelmo/.env`
2. **Project**: `<project>/.valksor/.env`

Project values override global values.

## Supported Variables

| Variable             | Description           |
|----------------------|-----------------------|
| `GITHUB_TOKEN`       | GitHub API token      |
| `GITLAB_TOKEN`       | GitLab API token      |
| `LINEAR_TOKEN`       | Linear API token      |
| `WRIKE_TOKEN`        | Wrike API token       |
| `ANTHROPIC_API_KEY`  | Anthropic API key     |
| `OPENAI_API_KEY`     | OpenAI API key        |

## Setting Variables

Create or edit the `.env` file:

```bash
# Global tokens (all projects)
mkdir -p ~/.valksor/kvelmo
cat >> ~/.valksor/kvelmo/.env << 'EOF'
GITHUB_TOKEN=ghp_xxxxxxxxxxxx
EOF

# Project-specific tokens
mkdir -p .valksor
cat >> .valksor/.env << 'EOF'
GITHUB_TOKEN=ghp_project_specific
EOF
```

## Using the CLI

```bash
# Set a token via CLI (saves to .env file)
kvelmo config set providers.github.token ghp_xxxxxxxxxxxx --scope global
kvelmo config set providers.github.token ghp_xxxxxxxxxxxx --scope project

# View current configuration
kvelmo config show
```

## Priority

Configuration sources are applied in this order (later overrides earlier):

1. Default values (lowest priority)
2. Global settings (`~/.valksor/kvelmo/kvelmo.yaml`)
3. Project settings (`<project>/.valksor/kvelmo.yaml`)
4. Global `.env` (`~/.valksor/kvelmo/.env`)
5. Project `.env` (`<project>/.valksor/.env`) (highest priority)

## Security Notes

- Never commit `.env` files to version control (add to `.gitignore`)
- Use file permissions to protect tokens: `.env` files are created with mode `0600`
- For CI/CD, inject tokens into `.env` files before running kvelmo:
  ```bash
  mkdir -p ~/.valksor/kvelmo
  echo "GITHUB_TOKEN=$GITHUB_TOKEN" >> ~/.valksor/kvelmo/.env
  chmod 600 ~/.valksor/kvelmo/.env
  ```
