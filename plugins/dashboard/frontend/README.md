# GoShimmer Dashboard

Programmed using modern web technologies.

### Dashboard in dev mode

There are two ways to launch dashboard in development mode: one using Docker, which doesn't require native installation
of any tools, and the other is running dashboard using natively installed tools. Docker approach has only been tested on
Linux.

#### Docker

1. Make sure to set `dashboard.dev` to true, to enable GoShimmer to serve assets from the webpack-dev-server.
2. Run `scripts/dashboard_dev_docker.sh` script. The script should be run from repository root directory. It will
   install all needed npm modules and run a webpack-dev-server instance. The script exposes dashboard on port `9090` by
   default, so it might be necessary to change it if port is already in use.
3. Set `dashboard.devDashboardAddress` config to IP of container running dashboard in dev mode. IP can be fetched by
   running: `docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' goshimmer-dashboard-dev`
4. Run Goshimmer docker-network. Using default port config, you should now be able to access the dashboard under
   http://127.0.0.1:8081

The Dashboard is hot-reload enabled.

#### Without Docker

1. Make sure to set `dashboard.dev` to true, to enable GoShimmer to serve assets from the webpack-dev-server.
2. Install all needed npm modules via `yarn install`.
3. Run a webpack-dev-server instance by running `yarn start` within the `frontend` directory.
4. Using default port config, you should now be able to access the dashboard under http://127.0.0.1:8081

The Dashboard is hot-reload enabled.

### Pack your changes

We are using [pkger](https://github.com/markbates/pkger) to wrap all built frontend files into Go files.

#### Docker

Docker will mount all necessary files and directories and will create all resulting files in the Goshimmer repository
folder. This approach has only been tested on Linux.

In order to build and package dashboard, run `scripts/pkger_docker.sh` from repository root directory.

#### Without Docker

1. [Install `pkger`](https://github.com/markbates/pkger#installation) if not already done.
2. Build Dashboard by running `yarn build` within the `frontend` directory.
3. Run `pkger`.
4. `pkged.go` under root directory of goShimmer should have been modified.
5. Done. Now you can build goShimmer and your Dashboard changes will be included within the binary.

Alternatively, all necessary commands are compiled into a script `script/pkger.sh` that builds and packages dashboard
using natively installed tools.