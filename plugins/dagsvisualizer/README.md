# GoShimmer DAGs visualizer

This project was bootstrapped with [Create React App](https://github.com/facebook/create-react-app).

## DAGs visualizer structure

### Front-end
DAGs visualizer is a React App that implemented in TypeScript.
Here are the main directories in `/src`:
* **components**: contains React components.
* **stores**: contains [stores](https://learn.co/lessons/react-stores) that will be injected to components via [`mobx`](https://mobx.js.org/README.html). Stores handle data from the back-end, and each DAG manages its own store.
* **models**: contains data structures of different types of websocket messages.
* **graph**: contains the interface and implementations of visualization, currently there are [VivaGraphJS](https://github.com/anvaka/VivaGraphJS) and [Cytoscape.js](https://js.cytoscape.org/).
* **styles**: contains files related to styling.
* **utils**: contains utility functions.

### Back-end
The `.go` files forms a plugin that register several events to collect data from each DAG and a back-end server for the webpage.

## DAGs visualizer in dev mode

Dev mode has only been tested on Linux.

### Docker
Run the yarn development server in a container and add it to the docker-network.

1. Make sure to set `dagsvisualizer.dev` to true, to enable GoShimmer to serve assets.
2. Make sure to set `dagsvisualizer.devBindAddress` to `dagsvisualizer-dev-docker:3000`.
3. Run Goshimmer docker-network.
4. Go to goshimmer root directory and run script `scripts/dags_visualizer_dev_docker.sh`. It will
   install all needed npm modules and create a container with a running development server instance.
5. Using default port config, you should now be able to access the DAGs visualizer under http://127.0.0.1:8061

To see the changes, you need to manually reload the page.

## Pack your changes

We are using [pkger](https://github.com/markbates/pkger) to wrap all built frontend files into Go files.

1. [Install `pkger`](https://github.com/markbates/pkger#installation) if not already done.
2. Build DAGs visualizezr by running `yarn build` within the `frontend` directory.
3. Run `pkger`.
4. `pkged.go` under root directory of goShimmer should have been modified.
5. Done. Now you can build goShimmer and your DAGs visualizer changes will be included within the binary.
