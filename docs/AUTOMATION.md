# Automation Items

Add back to release.yml and complete workflow once ready to automate.

TODO:
- Script to bump build.yaml charts image version/unRC it
- Script/wrapper to call `go generate` in `r/r` correctly
- (Optional) Auto unRC of chart deps in `build.yaml`


```
  create-rancher-pr:
    needs: [parse-tag, publish-release]
    if: success() && needs.parse-tag.outputs.build_type == 'prod'
    runs-on: ubuntu-latest
    steps:
      - name: Checkout
        uses: actions/checkout@3d3c42e5aac5ba805825da76410c181273ba90b1 # v7.0.1

      - name: Get `r/r` PR token
        uses: rancher-eio/read-vault-secrets@0da85151ad1f19ed7986c41587e45aac1ace74b6 # v3
        with:
          secrets: |
            github/token/rancher--rancher--pull_requests--write token | GH_APP_TOKEN

      - name: Get Rancher target branch
        id: rancher
        run: |
          RANCHER_MINOR=${{ needs.parse-tag.outputs.rancher_minor }}
          RANCHER_BRANCH=$(yq eval ".chart-versions.\"${RANCHER_MINOR}\".rancher-branch" config.yaml)
          echo "branch=$RANCHER_BRANCH" >> $GITHUB_OUTPUT

      # TODO generate actual PR changes
      # Consider updating all charts refs to unRC at the same time

      - name: Create PR to rancher/rancher
        run: |
          VERSION="${{ needs.parse-tag.outputs.version }}"
          IMAGE="rancher/rancher-assets:${VERSION}"

          PR_BODY=$(.github/scripts/generate-rancher-pr-body.sh \
            "$VERSION" \
            "${{ needs.parse-tag.outputs.rancher_minor }}" \
            "$IMAGE")

          gh pr create --repo rancher/rancher \
            --base ${{ steps.rancher.outputs.branch }} \
            --title "Update to rancher-assets ${VERSION}" \
            --body "$PR_BODY"
        env:
          GH_TOKEN: ${{ env.GH_APP_TOKEN }}
```