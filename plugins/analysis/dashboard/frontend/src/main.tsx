import App from "./app/App";
import { AutopeeringStore } from "./app/stores/AutopeeringStore";
import { Provider } from "mobx-react";
import React from "react";
import ReactDOM from "react-dom";
import { Route } from "react-router";
import { BrowserRouter as Router } from "react-router-dom";
import "./main.scss";
import ManaStore from "./app/stores/ManaStore";

export const autopeeringStore = new AutopeeringStore();
export const manaStore = new ManaStore();

const stores = {
    "autopeeringStore": autopeeringStore,
    "manaStore": manaStore,
};

// render react DOM
ReactDOM.render(
    <Provider {...stores}>
        <Router>
            <Route component={(props) => <App {...props} />} />
        </Router>
    </Provider>,
    document.getElementById("root")
);