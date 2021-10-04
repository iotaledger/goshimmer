---
description: How to create a GoShimmer release.  
image: /img/logo/goshimmer_light.png
keywords:
- github
- release
- banner version
- Changelog
- build
- node
- newest image
---
# How to Do a Release

1. Create a PR into `develop` updating the banner version (`plugins/banner.AppVersion`) and mentioning the changes in `CHANGELOG.md`.
2. Create a PR merging `develop` into `master`.
3. Go to release workflow https://github.com/iotaledger/goshimmer/actions/workflows/release.yml and click the gray "Run workflow" button to configure the release process.
4. In "Branch" field set `master`, in "Tag name" set current version, in "Release description" paste the changes recently added to `CHANGELOG.md`. Click the green "Run workflow" to trigger the automatic release and deployment process.
5. Check that the binaries are working.
6. Check that the nodes are up and functioning on `devnet`__.