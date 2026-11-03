# lock.yaml

`lock.yaml` records every input that goes into the generated Dockerfiles, so a build is reproducible from any commit. It is generated - never edit it by hand.

For the release pipeline, see [RELEASE-TEAM-GUIDE.md](RELEASE-TEAM-GUIDE.md).

## Structure

```yaml
chart-versions:
  "2.16":
    upstream-refs:
      prod:                       # Used by Dockerfile.2.16
        charts:  { branch: release-v2.16, commit: fe1df8bf..., fetched-at: ... }
        partner: { branch: main,          commit: 6b7da8a4..., fetched-at: ... }
        rke2:    { branch: main,          commit: cf8a16be..., fetched-at: ... }
      dev:                        # Used by Dockerfile.2.16-dev
        charts:  { branch: dev-v2.16,     commit: 82963a9b..., fetched-at: ... }
        partner: { branch: main,          commit: 6b7da8a4..., fetched-at: ... }
        rke2:    { branch: main,          commit: cf8a16be..., fetched-at: ... }
base-image:                       # Synced from config.yaml
  bci-base: 16.0@sha256:...
  bci-micro: 16.0@sha256:...
copy-script-hash: 3711...         # SHA-256 of package/copy-charts.sh
template-hashes:                  # SHA-256 of internal/generator/tmpl/*.tmpl
  prod-template: 0f5d...
  dev-template: 2067...
generated-at: ...
generator-version: v0.1.0
```

Branches come from `config.yaml` (`chart-versions.<minor>.prod|dev`). `fetched-at` only changes when a ref's branch or commit changes.

## How It Gets Updated

| Command | Upstream refs | Hashes, base image, Dockerfiles |
|---------|---------------|--------------------------------|
| `make update` | Queries the latest commit of every configured branch | Recomputed and regenerated |
| `make generate` | Kept as they are | Recomputed and regenerated |

- **Scheduled:** `pr-builder-cron.yml` runs `make update` every Monday and Wednesday at 11:00 UTC and opens an auto-merged PR when `lock.yaml` or `dockerfiles/` changed.
- **Manual:** run `make update` to pick up upstream now, or `make generate` after changing `config.yaml`, templates or `package/copy-charts.sh`. Commit `lock.yaml` and `dockerfiles/` together.

## What a Change Releases

Merging a `lock.yaml` or `dockerfiles/` change to `main` runs `auto-release.yml`:

- Minors whose rendered Dockerfile changed get a tag. A `copy-script-hash` change releases every minor, since the script is copied into the image without appearing in the Dockerfile.
- **prod** if that minor's `upstream-refs.prod.charts.commit` moved, **dev** otherwise.

## FAQ

### My upstream commit isn't in the image

The lock file still points at an older commit. Wait for the next scheduled run, dispatch `pr-builder-cron.yml`, or run `make update` and open a PR.

### Which upstream commits are in a released image?

```bash
git show v2.15-20260805T1600Z:lock.yaml
docker inspect rancher/rancher-assets:v2.15-20260805T1600Z | jq '.[0].Config.Labels'
# io.rancher.charts.commit, io.rancher.partner.commit, io.rancher.rke2.commit, io.rancher.build-type
```

### Can I pin a specific upstream commit?

No. `make update` always takes the head of the configured branch.
