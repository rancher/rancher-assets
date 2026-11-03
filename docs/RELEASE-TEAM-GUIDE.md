# Release Team Guide

Quick reference for managing rancher-assets releases.

## TL;DR

- **Scheduled releases** run every Monday and Wednesday at 11:00 UTC, fully automated from upstream charts to rancher/rancher
- **Auto-releases** tag every Rancher minor whose inputs changed on merge to main (prod only when rancher-charts' prod commit moved, dev otherwise)
- **Manual releases** are a CalVer tag pushed by hand, or a `workflow_dispatch` of a scheduled step
- **PR smoke tests** validate builds before merge
- **Git tags** (`v*`) trigger builds automatically
- **CalVer** versions with minute-level timestamp granularity

## Version Format

**CalVer with ISO 8601 Timestamp:**

`v{RANCHER_MINOR}-{YYYYMMDD}T{HHMM}Z[-dev]`

**Examples:**
- `v2.14-20260724T1430Z` - Prod release for Rancher 2.14
- `v2.15-20260724T1431Z-dev` - Dev release for Rancher 2.15

**Key Points:**
- Rancher version is explicit in tag
- Timestamp auto-generates (no version bumping)
- Multiple releases per day supported
- `-dev` suffix indicates dev build (rancher-charts `dev-v2.X`)
- No suffix indicates prod build (rancher-charts `release-v2.X`)
- partner-charts and rke2-charts track `main` for both

## Workflow Overview

### 1. PR Smoke Test (automatic)

**Trigger:** Pull request touching `dockerfiles/**`, `lock.yaml`, `config.yaml`, `Makefile` or `hack/make/**`

**What it does:**
- Detects which Dockerfiles changed
- Builds each changed Dockerfile for `linux/amd64` and `linux/arm64` (no push)
- Reports pass/fail as PR check

**Purpose:** Catch build failures before merge

### 2. Auto-Release (automatic)

**Trigger:** Merge to `main` with `dockerfiles/**` or `lock.yaml` changes

**What it does:**
- Detects which Rancher minors have changed inputs
- Creates one CalVer tag per changed minor:
  - **prod** when rancher-charts' prod commit moved for that minor in `lock.yaml`
  - **dev** for anything else (partner/rke2 chart moves, base image or script changes)
- Pushes all tags atomically → triggers parallel builds

**Example:**
A merge where rancher-charts `release-v2.14` moved, and only partner-charts moved for 2.15, creates:
- `v2.14-20260724T1530Z` (prod)
- `v2.15-20260724T1530Z-dev` (dev)

A prod release for any other reason is a manual tag push (see [Manual Releases](#manual-releases)).

### 3. Build and Release (automatic)

**Trigger:** Any `v*` tag

**What it does:**
- Builds the tagged Dockerfile and pushes images to Docker Hub and the Prime staging registry (prod tags also to the Prime registry)
- Publishes a GitHub release (dev tags as prerelease)

### 4. Push to rancher/rancher (automatic)

**Trigger:** GitHub release published (prod and dev)

**What it does:**
- Opens a PR in rancher/rancher bumping `defaultAssetsImage` in `build.yaml`, on the branch mapped to the tag's minor
- Requests auto-merge; rancher/rancher's `auto-approve-rancher-assets.yml` approves it

## Manual Releases

`release.yml` builds any `v*` tag, so a manual release is a CalVer tag pushed by hand with `scripts/create-tags-local.sh`:

```bash
# Auto-detect targets from origin/main..HEAD (same rules as auto-release)
./scripts/create-tags-local.sh --push

# Specific Dockerfile (prod or dev is chosen by the file)
./scripts/create-tags-local.sh dockerfiles/Dockerfile.2.15 --push

# Specific commit
./scripts/create-tags-local.sh --commit abc123 dockerfiles/Dockerfile.2.14 --push
```

Omit `--push` to review the tags locally first.

**When to use:**
- Rebuild without Dockerfile changes (new timestamp)
- Prod release that auto-release classified as dev
- Release from an older commit
- Emergency releases

## Release Cadence

Releases are fully automated. Every Monday and Wednesday at 11:00 UTC, new upstream chart commits flow end to end into rancher/rancher with no human intervention:

| Step | Workflow | Trigger |
|------|----------|---------|
| 1. Bump chart references, open PR | `pr-builder-cron.yml` | Cron `0 11 * * 1,3` |
| 2. Approve and merge bump PR | `auto-merge-bump-pr.yml` | PR labeled `status/auto-created` |
| 3. Tag changed minors | `auto-release.yml` | Push to `main` (`dockerfiles/**`, `lock.yaml`) |
| 4. Build and publish images | `release.yml` | `v*` tag |
| 5. Open bump PR in rancher/rancher | `push-to-rancher.yml` | Release published |
| 6. Approve, re-run flaky checks, auto-merge | rancher/rancher `auto-approve-rancher-assets.yml` | Cron `0 0,8,16 * * 1-5` |

### Forcing a run

Any scheduled step can be triggered manually from the Actions tab (`workflow_dispatch`):

- **`pr-builder-cron.yml`** - pick up upstream changes now instead of waiting for the next Monday/Wednesday. Input: `target_branch` (default `main`).
- **`push-to-rancher.yml`** - re-push an existing release to rancher/rancher. Inputs: `git_ref` (tag, SHA or branch), `rancher_minor` (e.g. `2.16`).
- **rancher/rancher `auto-approve-rancher-assets.yml`** - approve pending bump PRs and re-run failed checks now.

## Common Workflows

### Scheduled Update (no action needed)

```
1. pr-builder-cron runs `make update` (Mon/Wed 11:00 UTC)
   → Opens "Bump chart references" PR if upstream moved
2. PR smoke test passes → PR auto-merged
3. Auto-release tags each changed minor (prod or dev)
   → Builds, GitHub releases, rancher/rancher bump PRs
```

### Engineering Change

```
1. Change internal/generator/tmpl/*.tmpl or config.yaml, run `make generate`
   (Dockerfiles are generated - never edit them by hand)
2. Open PR → smoke test builds affected Dockerfiles
3. Merge → auto-release tags affected minors as dev
4. Need prod? Push a prod tag (see Manual Releases)
```

### Emergency Rebuild (No Dockerfile Changes)

```
./scripts/create-tags-local.sh dockerfiles/Dockerfile.2.15 --push
→ New v2.15-{timestamp} tag → build, release, rancher/rancher PR
```

### Security Fix (BCI Base Image CVE)

```
1. Update `base-image` in config.yaml, run `make generate`
2. Open PR → smoke test builds all Dockerfiles
3. Merge → auto-release tags all minors as dev
4. Push prod tags for each affected minor:
   ./scripts/create-tags-local.sh dockerfiles/Dockerfile.2.14 dockerfiles/Dockerfile.2.15 --push
```

## Query Versions

### Latest prod for Rancher 2.15
```bash
git tag -l "v2.15-*" --sort=-version:refname | grep -v -- '-dev$' | head -1
```

### Latest dev for Rancher 2.14
```bash
git tag -l "v2.14-*" --sort=-version:refname | grep -- '-dev$' | head -1
```

### All releases for Rancher 2.15
```bash
git tag -l "v2.15-*" --sort=-version:refname
```

### When was a version released?
```bash
git log -1 --format='%ai' v2.15-20260724T1530Z
```

## Decision Matrix

> Minors and commit SHAs below are illustrative examples only.

| Situation | Action |
|-----------|--------|
| Upstream charts moved | Nothing - scheduled run handles it |
| Need upstream changes before next Mon/Wed | Dispatch `pr-builder-cron.yml` |
| Need 2.15 prod without changes | `./scripts/create-tags-local.sh dockerfiles/Dockerfile.2.15 --push` |
| Need 2.14 dev rebuild | `./scripts/create-tags-local.sh dockerfiles/Dockerfile.2.14-dev --push` |
| Release 2.14 prod from commit abc123 | `./scripts/create-tags-local.sh --commit abc123 dockerfiles/Dockerfile.2.14 --push` |
| Re-push a release to rancher/rancher | Dispatch `push-to-rancher.yml` |

## Build Type Differences

### Prod
- Uses `Dockerfile.{MINOR}` (no -dev suffix)
- rancher-charts `release-v{MINOR}`
- Tag format: `v{MINOR}-{timestamp}` (no -dev)
- Auto-released only when rancher-charts' prod commit moved; otherwise push the tag manually
- For production Rancher releases

### Dev
- Uses `Dockerfile.{MINOR}-dev`
- rancher-charts `dev-v{MINOR}`
- Tag format: `v{MINOR}-{timestamp}-dev`
- Published as a GitHub prerelease
- For testing and development

Both build types open a bump PR in rancher/rancher.

## Credentials

No personal access tokens are used. Workflows read credentials from Vault (`rancher-eio/read-vault-secrets`, GitHub OIDC) under `secret/data/github/repo/rancher/rancher-assets/`:

| Vault path | Used by | Purpose |
|------------|---------|---------|
| `github/pr-actions-write-app/credentials` | `pr-builder-cron.yml`, `auto-release.yml`, `push-to-rancher.yml` | GitHub App token: open PRs, push tags, open rancher/rancher PRs |
| `dockerhub/rancher/credentials` | `release.yml` | Push images to Docker Hub |
| `rancher-prime-stg-registry/credentials` | `release.yml` | Push images to Prime staging |
| `rancher-prime-registry/credentials` | `release.yml` (prod only) | Push images to Prime |

`auto-merge-bump-pr.yml` uses repository secrets `AUTO_APPROVE_APP_ID` and `AUTO_APPROVE_PRIVATE_KEY` (approves and merges bump PRs).

## Monitoring

### Check Scheduled Bump
- **Where**: Actions → PR Builder Cron
- **When**: Mon/Wed 11:00 UTC, or after dispatch
- **Shows**: Whether upstream moved and the bump PR opened

### Check Auto-Release
- **Where**: Actions → Auto Release
- **When**: After merge to main with `dockerfiles/**` or `lock.yaml` changes
- **Shows**: Release targets (minor + prod/dev), tags created

### Check Builds
- **Where**: Actions → Build and Release
- **When**: After tag creation
- **Shows**: Build status, image push, release publication

### Check rancher/rancher PRs
- **Where**: Actions → Push to rancher/rancher
- **When**: After a GitHub release is published
- **Shows**: Target branch, bump PR link, auto-merge request

### Check PR Smoke Tests
- **Where**: PR checks
- **When**: On pull request
- **Shows**: Which Dockerfiles built successfully

### Verify Tags
```bash
git tag -l "v2.*" --sort=-version:refname | head -20
```

### GitHub Releases
- **Where**: Repository → Releases tab
- **Shows**: All published releases with artifacts

## Troubleshooting

### "No Dockerfiles changed" in auto-release
- Auto-release only triggers on `dockerfiles/**` changes
- If you changed lock.yaml but Dockerfiles didn't update, run `make generate`
- Use manual release if needed

### "Tag already exists"
- Each CalVer timestamp is unique per build type
- If you need another release, wait 1 minute (timestamp will change)
- Or use manual release from a different commit

### Build fails
- Check `release.yml` workflow logs
- Verify Dockerfile exists and is valid
- Check `lock.yaml` has required upstream refs
- Ensure base images are accessible

### PR to rancher/rancher not created
- Only happens for prod builds (not dev)
- Check `RANCHER_REPO_TOKEN` secret is configured
- Verify `rancher-branch` in `config.yaml` is correct

### PR smoke test fails
- Check which Dockerfile failed in PR checks
- Review build logs
- Fix Dockerfile and push to PR
- Smoke test re-runs automatically

## Tips

**Multiple releases per day**: Fully supported. CalVer includes hour:minute.

**Both prod and dev at once**: Possible via manual release (run workflow twice with different build_type)

**Selective release**: Use manual release with specific `rancher_minor`

**Rebuild everything**: Manual release with empty `rancher_minor` and chosen `build_type`

**Testing**: Create dev releases first, validate, then create prod releases

## Contact

Questions or issues:
- Open issue in rancher/rancher-assets
- Tag @rancher/release-team

## Setup Requirements (Delete this section after setup)

The workflows require these GitHub secrets to be configured:

**`WORKFLOW_PAT`** (Required for automated releases)
- Personal Access Token - **MUST be a Classic token** (fine-grained tokens may not trigger workflows reliably)
- Scope: **`repo`** (Full control of private repositories)
- Used ONLY to push tags that trigger build workflows
- Without this, tags are created but builds won't trigger
- Fallback: Uses `GITHUB_TOKEN` (creates tags but doesn't trigger builds)
- Note: `GITHUB_TOKEN` is used for all other operations

**To create WORKFLOW_PAT:**
1. GitHub Settings → Developer settings → Personal access tokens → Tokens (classic)
2. Generate new token (classic)
3. Select scope: `repo` ✅
4. Copy token
5. Add to Repository Settings → Secrets → New repository secret → Name: `WORKFLOW_PAT`

**Important:** Fine-grained PATs may not work for triggering workflows. Use classic PAT with `repo` scope.

**`RANCHER_REPO_TOKEN`** (Optional - for rancher/rancher PRs)
- PAT with access to create PRs in `rancher/rancher`
- Only needed if you want automated PRs on stable releases
- If not set, stable releases won't create rancher/rancher PRs

**Configure in:** Repository Settings → Secrets and variables → Actions → New repository secret
