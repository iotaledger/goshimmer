# How to do a release

1. Create a PR into `develop` updating the banner version (`plugins/banner.AppVersion`) and mentioning the changes in `CHANGELOG.md`
2. Create a PR merging `develop` into `master`
3. Create a release via the release page with the same changelog entries as in `CHANGELOG.md` for the given version tagging the `master` branch
4. Pray that the CI gods let the build pass
5. Check that the binaries are working
6. Stop the entry-node
7. Delete DB
8. Update version in docker-compose
9. Pull newest image
10. Start the node