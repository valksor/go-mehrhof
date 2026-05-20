# test-provider

Test a provider connection to verify it is configured and reachable.

## Usage

```bash
kvelmo test-provider <provider>
```

## Description

Verifies that a task provider (GitHub, GitLab, Linear, Wrike, Jira, Azure DevOps) has valid credentials configured and the remote service is reachable. Useful for troubleshooting authentication issues or validating token setup.

The command connects to the global socket and calls `providers.test` with the specified provider name. It reports whether the connection succeeded and includes any detail from the provider.

## Arguments

| Argument     | Description                                                                 |
| ------------ | --------------------------------------------------------------------------- |
| `<provider>` | Provider name: `github`, `gitlab`, `linear`, `wrike`, `jira`, `azuredevops` |

## Examples

### Test GitHub connection

```bash
kvelmo test-provider github
```

Output on success:

```
github: connected
  Authenticated as octocat
```

Output on failure:

```
github: failed
  401 Unauthorized
```

### Test GitLab connection

```bash
kvelmo test-provider gitlab
```

### Test Linear connection

```bash
kvelmo test-provider linear
```

## Prerequisites

- The global socket must be running (`kvelmo serve`)
- The provider must have a token configured (see [login](/cli/login.md))

## Taxonomy Note

`test-provider` is a standalone command that takes a provider name as an argument. It is related to provider configuration, but it is not a `config` subcommand.

## Web UI

Provider connections can also be tested from the Settings modal under the Providers section using the "Test" buttons.

## Related

- [login](/cli/login.md) — Configure provider authentication
- [start](/cli/start.md) — Load a task from a provider
- [Providers](/providers/index.md) — Provider documentation
