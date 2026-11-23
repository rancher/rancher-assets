# bump-pr-builder

Automates opening PRs against this repository when `make update` detects changes to chart references or generated files.

## Workflows

### `.github/workflows/pr-builder-cron.yml`

Scheduled workflow that:
- **Runs on schedule** via cron (11:00 UTC Monday and Wednesday)
- **Manually triggered** via `workflow_dispatch`
- **Triggered by rancher/charts** via `repository_dispatch` of type `charts-staged-pinned`: a workflow of `staging` sends it
  when a chart that Rancher pins (see `push-to-rancher/update-pins.sh`) is added to `staging`, so a new version of it does
  not wait for the schedule. The schedule stays: it also covers the charts that are not pinned, and a dispatch that failed.
  Nothing happens when the dispatch finds no change (step 4 below). The sender waits 20 minutes after the last merge before
  it sends, so two dispatches are never close enough to open two bump PRs for the same change: this workflow has no
  concurrency group and does not look for a bump PR that is still open, and a bump PR takes some minutes to be merged.

## How it works

1. Run `make update` to update chart references and regenerate files
2. Check if changes exist (using `./scripts/verify`)
3. If changes detected:
   - Commit changes to a timestamped feature branch
   - Push branch and create PR against the current branch (typically `main`)
4. If no changes, exit early (no-op)

## Target branch

By default, PRs are created against the branch the workflow is running on (typically `main`). This can be overridden via:

```bash
export TARGET_BRANCH="release/v2.15"
```

## Local usage

### Prerequisites

- **Bash**: All scripts require `bash` (not POSIX sh). They use bash-specific features like arrays, `[[`, and `BASH_SOURCE` for safety and clarity.
- **gh**: GitHub CLI for creating PRs
- **git**: For committing and pushing changes
- **Docker** or **CI environment**: For running `make update` (uses CI image unless `CI=true`)

### Running locally

```bash
./.github/workflows/bump-pr-builder/run-local.sh \
  [--dry-run] \
  [--remote origin] \
  [--target-branch main]
```

Options:
- `--dry-run` - runs all local git work (commits to your repo) but skips push and PR creation
- `--remote` - git remote name (default: `origin`)
- `--target-branch` - target branch for PR (default: current branch or `main`)

## Step sequence

| Script | What it does |
|---|---|
| `common.sh` | Shared functions and environment setup for all scripts |
| `run-update.sh` | Runs `make update` and checks for changes |
| `create-pr.sh` | Commits changes, pushes branch, creates PR |
| `run-gha.sh` | GHA entry point: validates environment, calls run-update.sh and create-pr.sh |
| `run-local.sh` | Local entry point: parses args, sets up env, calls run-update.sh and create-pr.sh |

## Key env vars

| Var | Description |
|---|---|
| `REPO_DIR` | Path to rancher-assets clone (default: repo root) |
| `REMOTE` | Git remote name (default: `origin`) |
| `TARGET_BRANCH` | Target branch for PR (default: current branch) |
| `DRY_RUN` | Set to `true` to skip push and PR creation |
| `SOURCE_REPO` | Source repo for PR body (default: `rancher/rancher-assets`) |

## GHA prerequisites

The workflow reads a GitHub App credential from Vault at:

```
secret/data/github/repo/rancher/rancher-assets/github/app-credentials
```

The app must have write access to `rancher/rancher-assets` to push branches and open PRs.

## Error handling

The workflow exits early (success) if no changes are detected. If `make update` fails, the workflow fails.

Common scenarios:
- No changes detected → exit 0 (no-op, expected behavior)
- `make update` fails → exit 1 (workflow fails)
- PR already exists for similar changes → PR creation may fail (workflow fails)
