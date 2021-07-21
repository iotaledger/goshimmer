# GoShimmer Analysis Dashboard

Programmed using modern web technologies.

## Start the Dashboard In Dev Mode

1. Make sure to set `analysis.dashboard.dev` to true, to enable GoShimmer to serve assets from the webpack-dev-server.
2. Install all the needed npm modules using the `yarn install` command.
3. Run a webpack-dev-server instance by running `yarn start` within the `frontend` directory.
4. Using default port config, you should now be able to access the analysis dashboard under http://127.0.0.1:8000

:::info
The Analysis Dashboard is hot-reload enabled.
:::

### Pack Your Changes

We use [pkger](https://github.com/markbates/pkger) to wrap all built frontend files into Go files. Please follow these instructions when packaging your changes:

1. If you haven't already done so, [Install `pkger`](https://github.com/markbates/pkger).
2. Check that the correct webpack-cli (version v3.3.11) is installed:
   
   1. Get your current installed version running `yarn webpack-cli --version`.
   2. If a newer version is installed, use `yarn remove webpack-cli` to remove it, and `yarn add webpack-cli@3.3.11` to add the correct version.
   
3. Build Analysis Dashboard by running `yarn build` within the `frontend` directory.
4. Navigate to the root of the repo.
5. Run `pkger` in the root of the repo.
6. `pkged.go` should have been modified.

You can now build GoShimmer, and your Analysis Dashboard changes will be included within the binary.

The above steps can also be done by running the `scripts/pkger.sh` script from the root folder.
