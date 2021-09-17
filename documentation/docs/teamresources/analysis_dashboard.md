---
description: How to use the Dashboard in dev mode and set up hot loading and packaging.
image: /img/logo/goshimmer_light.png
keywords:
- port config
- pkger
- webpack
- build
- change
- npm
- yarn
---
# GoShimmer Analysis Dashboard

Programmed using modern web technologies.

### Dashboard in Dev Mode

1. Make sure to set `analysis.dashboard.dev` to true, to enable GoShimmer to serve assets
   from the webpack-dev-server.
2. Install all needed npm modules via `yarn install`.
3. Run a webpack-dev-server instance by running `yarn start` within the `frontend` directory.
4. Using default port config, you should now be able to access the analysis dashboard under http://127.0.0.1:8000

The Analysis Dashboard is hot-reload enabled.

### Pack Your Changes

We are using [pkger](https://github.com/markbates/pkger) to wrap all built frontend files into Go files.

1. [Install `pkger`](https://github.com/markbates/pkger) if not already done.
2. Check that the correct webpack-cli (version v3.3.11) is installed: 

   2.1 `yarn webpack-cli --version`

   2.2 If a newer version is installed use `yarn remove webpack-cli` and `yarn add webpack-cli@3.3.11` 
3. Build Analysis Dashboard by running `yarn build` within the `frontend` directory.
4. Navigate to the root of the repo.
5. Run `pkger` in the root of the repo.
6. `pkged.go` should have been modified.
7. Done. Now you can build GoShimmer and your Analysis Dashboard changes will be included within the binary.

The above steps can also be done by running the `scripts/pkger.sh` script from the root folder.
