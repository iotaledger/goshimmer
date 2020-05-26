# GoShimmer Analysis Dashboard

Programmed using modern web technologies.

### Dashboard in dev mode

1. Make sure to set `analysis.dashboard.dev` to true, to enable GoShimmer to serve assets
   from the webpack-dev-server.
2. Install all needed npm modules via `yarn install`.
3. Run a webpack-dev-server instance by running `yarn start` within the `frontend` directory.
4. Using default port config, you should now be able to access the analysis dashboard under http://127.0.0.1:8000

The Analysis Dashboard is hot-reload enabled.

### Pack your changes

We are using [packr2](https://github.com/gobuffalo/packr/tree/master/v2) to wrap all built frontend files into Go files.

1. [Install `packr2`](https://github.com/gobuffalo/packr/tree/master/v2#binary-installation) if not already done.
2. Build Analysis Dashboard by running `yarn build` within the `frontend` directory.
3. Change to the `plugins/analysis/dashboard` directory.
4. Run `packr2`.
5. `plugins/analysis/dashboard/packrd` should have been modified.
6. Done. Now you can build goShimmer and your Analysis Dashboard changes will be included within the binary.
