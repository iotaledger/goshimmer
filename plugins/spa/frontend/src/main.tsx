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

// prepare MobX stores
const routerStore = new RouterStore();
const nodeStore = new NodeStore();
const explorerStore = new ExplorerStore(routerStore);
const stores = {
    "routerStore": routerStore,
    "nodeStore": nodeStore,
    "explorerStore": explorerStore,
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
