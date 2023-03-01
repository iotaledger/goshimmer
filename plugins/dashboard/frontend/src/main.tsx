import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Provider} from 'mobx-react';
import {createBrowserHistory} from 'history';
import 'chartjs-plugin-streaming';
import {App} from 'app/App';
import {RouterStore, syncHistoryWithStore} from 'mobx-react-router';
import {Router} from 'react-router-dom';
import NodeStore from "app/stores/NodeStore";
import ExplorerStore from "app/stores/ExplorerStore";
import FaucetStore from "app/stores/FaucetStore";
import VisualizerStore from "app/stores/VisualizerStore";
import ManaStore from "app/stores/ManaStore";
import {SlotStore} from "app/stores/SlotStore";
import ConflictsStore from "app/stores/ConflictsStore";

// prepare MobX stores
const routerStore = new RouterStore();
const nodeStore = new NodeStore();
const explorerStore = new ExplorerStore(routerStore);
const conflictsStore = new ConflictsStore(routerStore, nodeStore);
const faucetStore = new FaucetStore(routerStore);
const visualizerStore = new VisualizerStore(routerStore);
const manaStore = new ManaStore();
const slotStore = new SlotStore();
const stores = {
    "routerStore": routerStore,
    "nodeStore": nodeStore,
    "explorerStore": explorerStore,
    "conflictsStore": conflictsStore,
    "faucetStore": faucetStore,
    "visualizerStore": visualizerStore,
    "manaStore": manaStore,
    "slotStore": slotStore
};

const browserHistory = createBrowserHistory();
const history = syncHistoryWithStore(browserHistory, routerStore);

// render react DOM
ReactDOM.render(
    <Provider {...stores}>
        <Router history={history}>
            <App history={history}/>
        </Router>
    </Provider>,
    document.getElementById('root')
);
