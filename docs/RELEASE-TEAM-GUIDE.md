# Release Team Guide

Quick reference for managing rancher-assets releases.

## TL;DR

- **Auto-releases** happen when Dockerfiles merge to main (creates tags for changed files)
- **Manual releases** give you full control via GitHub Actions UI
- **PR smoke tests** validate builds before merge
- **Git tags** trigger builds automatically
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
- `-dev` suffix indicates dev build (uses `dev-v2.X` upstream branches)
- No suffix indicates prod build (uses `release-v2.X` upstream branches)

## Workflow Overview

### 1. PR Smoke Test (automatic)

**Trigger:** Pull request touching `dockerfiles/**`

**What it does:**
- Detects which Dockerfiles changed
- Builds each changed Dockerfile (no push)
- Reports pass/fail as PR check

**Purpose:** Catch build failures before merge

### 2. Auto-Release (automatic)

**Trigger:** Merge to `main` with `dockerfiles/**` changes

**What it does:**
- Detects which Dockerfiles changed
- Creates individual CalVer tag for EACH changed Dockerfile
- Pushes all tags → triggers parallel builds

**Example:**
If you merge a PR that changes:
- `Dockerfile.2.14` (prod)
- `Dockerfile.2.14-dev` (dev)
- `Dockerfile.2.15` (prod)
- `Dockerfile.2.15-dev` (dev)

Auto-release creates 4 tags:
- `v2.14-20260724T1530Z`
- `v2.14-20260724T1530Z-dev`
- `v2.15-20260724T1530Z`
- `v2.15-20260724T1530Z-dev`

All 4 build in parallel → 4 GitHub releases

### 3. Manual Release (manual)

**Trigger:** GitHub Actions UI → Manual Release workflow

**When to use:**
- Rebuild without Dockerfile changes (new timestamp)
- Release specific minor(s) only
- Release from older commit
- Emergency releases
- Testing

**Inputs:**
- `rancher_minor` - Specific version (e.g., "2.15") or empty for ALL
- `build_type` - "dev" or "prod"
- `commit_sha` - Specific commit or empty for HEAD

**Examples:**

Release Rancher 2.15 prod:
```
rancher_minor: 2.15
build_type: prod
commit_sha: [empty]
```

Release ALL minors as dev:
```
rancher_minor: [empty]
build_type: dev
commit_sha: [empty]
```

Release from specific commit:
```
rancher_minor: 2.14
build_type: prod
commit_sha: abc123def456
```

## Common Workflows

### Normal Update Flow

```
1. Run `make generate` locally
   → Updates lock.yaml and all Dockerfiles

2. Create PR with changes
   → PR smoke test validates all builds

3. PR approved and merged
   → Auto-release creates 4 tags (2.14 prod/dev, 2.15 prod/dev)
   → 4 parallel builds
   → 4 GitHub releases
   → Prod releases create rancher/rancher PRs
```

### Manual Engineering Change

```
1. Engineer fixes bug in Dockerfile.2.14 only
   → Creates PR changing one file

2. PR smoke test validates
   → Builds Dockerfile.2.14 only

3. PR merged
   → Auto-release creates v2.14-{timestamp}
   → Single build and release
```

### Emergency Rebuild (No Dockerfile Changes)

```
1. Need new release but Dockerfiles haven't changed
   → Use manual release workflow

2. GitHub UI → Actions → Manual Release
   rancher_minor: 2.15
   build_type: prod
   commit_sha: [empty for HEAD]

3. Creates v2.15-{new-timestamp}
   → Builds and releases
```

### Security Fix All Versions

```
1. CVE in BCI base image
   → Update BCI version in config
   → Run `make generate`
   → All Dockerfiles update

2. Create PR
   → PR smoke test builds all 4 Dockerfiles

3. Merge PR
   → Auto-release creates 4 tags
   → All versions rebuild with fixed base image

4. All 4 releases ready for consumption
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

| Situation | Action | rancher_minor | build_type |
|-----------|--------|---------------|------------|
| Dockerfiles merged to main | Auto (wait) | N/A | N/A |
| Need 2.15 prod without changes | Manual | `2.15` | `prod` |
| Need ALL dev rebuilds | Manual | `` (empty) | `dev` |
| Emergency 2.14 prod | Manual | `2.14` | `prod` |
| Test build from commit abc123 | Manual | `2.14` | `dev` + commit |

## Build Type Differences

### Prod (`build_type: prod`)
- Uses `Dockerfile.{MINOR}` (no -dev suffix)
- Uses upstream `release-v{MINOR}` branches
- Tag format: `v{MINOR}-{timestamp}` (no -dev)
- Creates PR to rancher/rancher
- For production Rancher releases

### Dev (`build_type: dev`)
- Uses `Dockerfile.{MINOR}-dev`
- Uses upstream `dev-v{MINOR}` branches
- Tag format: `v{MINOR}-{timestamp}-dev`
- No rancher/rancher PR
- For testing and development

## Required Secrets

### `GITHUB_TOKEN` (automatic)
- Provided by GitHub Actions
- Used for most operations

### `RANCHER_REPO_TOKEN` (optional)
- PAT with access to `rancher/rancher` repository
- Required only if you want automated PRs on prod releases
- Scope: `repo` or `public_repo`
- If not set, prod releases succeed but skip PR creation

**To configure:**
1. GitHub Settings → Developer settings → Personal access tokens
2. Generate token with `repo` scope
3. Repository Settings → Secrets → New secret → `RANCHER_REPO_TOKEN`

## Monitoring

### Check Auto-Release
- **Where**: Actions → Auto Release
- **When**: After merge to main with Dockerfile changes
- **Shows**: Which Dockerfiles changed, tags created

### Check Builds
- **Where**: Actions → Build and Release
- **When**: After tag creation
- **Shows**: Build status, image push, release publication

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
