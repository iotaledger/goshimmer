# GoShimmer DAGs visualizer

This project was bootstrapped with [Create React App](https://github.com/facebook/create-react-app).

## DAGs visualizer structure

### Front-end
DAGs visualizer is a React App that implemented in TypeScript.
Here are the main directories in `/src`:
* **components**: contains React components.
* **stores**: contains [stores](https://learn.co/lessons/react-stores) that will be injected to components via [`mobx`](https://mobx.js.org/README.html). Stores handle data from the back-end, and each DAG manages its own store.
* **styles**: contains files related to styling.

### Back-end
The `.go` files forms a plugin that register several events to collect data from each DAG and a back-end server for the webpage.

## Dashboard in dev mode

Dev mode has only been tested on Linux.

### Docker
In docker dev mode, the master node query the yarn development server for webpage resources.

1. Make sure to set `dagsvisualizer.dev` to true, to enable GoShimmer to serve assets.
2. Make sure to set `dagsvisualizer.devBindAddress` to your host machine IP address with port `3000`, for example: `36.237.190.25:3000`. You can get you host machine IP with `ifconfig`.
3. Install all needed npm modules via `yarn install` within the `frontend` directory.
4. Run development server with `yarn start`
3. Run Goshimmer docker-network. Using default port config, you should now be able to access the DAGs visualizer under http://127.0.0.1:8061

To see the changes, you need to manually reload the page.

## Pack your changes

We are using [pkger](https://github.com/markbates/pkger) to wrap all built frontend files into Go files.

1. [Install `pkger`](https://github.com/markbates/pkger#installation) if not already done.
2. Build DAGs visualizezr by running `yarn build` within the `frontend` directory.
3. Run `pkger`.
4. `pkged.go` under root directory of goShimmer should have been modified.
5. Done. Now you can build goShimmer and your DAGs visualizer changes will be included within the binary.
