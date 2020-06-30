import App from 'app/App';
import AutopeeringStore from "app/stores/AutopeeringStore";
import FPCStore from "app/stores/FPCStore";
import { Provider } from 'mobx-react';
import * as React from 'react';
import * as ReactDOM from 'react-dom';
import { Route } from 'react-router';
import { BrowserRouter as Router } from 'react-router-dom';
import "./main.scss";

const fpcStore = new FPCStore();
const autopeeringStore = new AutopeeringStore()
const stores = {
    "fpcStore": fpcStore,
    "autopeeringStore": autopeeringStore,
};

// render react DOM
ReactDOM.render(
    <Provider {...stores}>
        <Router>
            <Route component={(props) => <App {...props} />} />
        </Router>
    </Provider>,
    document.getElementById('root')
);