# GoShimmer DAGs visualizer
The DAGs visualizer is our all round tool for visualizing DAGs. Be it Tangle, UTXO-DAG or Branch-DAG or their interactions. The DAGs visualizer is our go-to tool for visualization.

## How to run

DAGs visualizer is already packed into `pkged.go`.
To run it just simply launch a goshimmer node, open browser and go to `http://localhost:8061`.

:::note

UTXO-DAG and Branch-DAG will check if there's any added or removed vertex every 10 seconds and rearrange vertices positions.

:::

[![DAGs visualizer Overview](/img/tooling/dags-visualizer.png "DAGs visualizer overview")](/img/tooling/dags-visualizer.png)

### Global Functions
Global functions are used to apply settings across DAGs or interact with them.

#### Set explorer URL
Each node in a graph can be selected to see its contained information, and they are navigated to the dashboard explorer for more details. You can change the url to the desired dashboard explorer, default is `http://localhost:8081`.

#### Search Vertex Within Time Intervals
You can check how Tangle, UTXO and branch DAG look like in a given timeframe.
Press "search" button, it will show you numbers of messages, transactions and branches found within the given timeframe. If you want to render them in graphs, push "render" button.

The branch DAG shows not just branches in the given time interval (colored in orage) but also the full history (colored in blue) to the master branch.

:::note

Drawing a large amount of transactions or branches may slow down the browser.

:::

[![DAGs visualizer Searching](/img/tooling/searching.png "DAGs visualizer searching")](/img/tooling/searching.png)


#### Select and center vertex across DAGs
You can see a selected message/transaction/branch and its corresponding message/transaction/branch in other DAGs! Here's an example of sync with the selected transaction, you can see the message and branch that contains the transaction are highlighted.

[![DAGs visualizer Syncing with TX](/img/tooling/sync-with-tx.png "DAGs visualizer sync with TX")](/img/tooling/sync-with-tx.png)

Another example of sync with the selected branch:
[![DAGs visualizer Syncing with branch](/img/tooling/sync-with-branch.png "DAGs visualizer sync with branch")](/img/tooling/sync-with-branch.png)

## How to run in dev mode

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

## How to pack changes to pkged.go

We are using [pkger](https://github.com/markbates/pkger) to wrap all built frontend files into Go files.

1. [Install `pkger`](https://github.com/markbates/pkger#installation) if not already done.
2. Build DAGs visualizezr by running `yarn build` within the `frontend` directory.
3. Run `pkger`.
4. `pkged.go` under root directory of goShimmer should have been modified.
5. Done. Now you can build goShimmer and your DAGs visualizer changes will be included within the binary.
