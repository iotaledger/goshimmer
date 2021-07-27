import { inject, observer } from "mobx-react";
import React, { ReactNode } from "react";
import { hot } from "react-hot-loader/root";
import { withRouter } from "react-router";
import { Link, Redirect, Route, Switch } from "react-router-dom";
import "./App.scss";
import { AppProps } from "./AppProps";
import Autopeering from "./components/Autopeering/Autopeering";
import Mana from "./components/Mana/Mana";

@inject("autopeeringStore")
@observer
class App extends React.Component<AppProps, unknown> {
    public componentDidMount(): void {
        this.props.autopeeringStore.connect();
    }

    public render(): ReactNode {
        return (
            <div className="root">
                <header>
                    <Link className="brand" to="/">
                        <img src="/assets/logo-header.svg" alt="IOTA 2.0 DevNet Analyzer" />
                        <h1>IOTA 2.0 DevNet Analyzer</h1>
                    </Link>
                    <div className="badge-container">
                        {!this.props.autopeeringStore.websocketConnected &&
                            <div className="badge">Not connected</div>
                        }
                    </div>
                    <nav>
                        <Link to="/autopeering">
                            Autopeering
                        </Link>
                        <Link to="/mana">
                            Mana
                        </Link>
                    </nav>
                </header>
                <Switch>
                    <Route path="/autopeering" component={Autopeering} />
                    <Route path="/mana" component={Mana} />
                    <Redirect to="/autopeering" />
                </Switch>
                {this.props.children}
            </div>
        );
    }
}

export default hot(withRouter(App));
