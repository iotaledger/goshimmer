# How To Do a Release

1. Create a PR targeting `develop`, updating the banner version (`plugins/banner.AppVersion`), and mentioning the changes in `CHANGELOG.md`.
2. Create a PR merging `develop` into `master`.
3. Create a release via the release page with the same changelog entries as in `CHANGELOG.md` for the given version tagging the `master` branch.
4. Await for the CI system approves the build pass.
5. Check that the binaries are working.
6. Stop the entry-node.
7. Delete the DB.
8. Update version in docker-compose.
9. Pull newest image.
10. Start the node.