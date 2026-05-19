# Security

Run security scans against the current project and review findings.

## Opening

Open the Security panel from the sidebar. It loads as a modal overlay.

## Running a Scan

Click **Scan** to trigger a security scan of the project directory. The scan checks for leaked secrets, vulnerable patterns, and other security issues.

## Findings

After a scan completes, results appear in a table:

| Column      | Description                                                      |
| ----------- | ---------------------------------------------------------------- |
| Severity    | `critical`, `high`, `medium`, or `low` — color-coded by severity |
| File        | Path to the affected file                                        |
| Line        | Line number of the finding                                       |
| Rule        | Rule identifier that triggered the finding                       |
| Description | Explanation of the security issue                                |

If the scan finds no issues, a success message confirms the project is clean.

## Re-scanning

Click **Scan** again at any time to re-run the scan with fresh results.
