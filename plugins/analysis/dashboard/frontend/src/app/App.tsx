import Autopeering from "app/components/Autopeering/Autopeering";
import Conflict from "app/components/FPC/Conflict";
import FPC from "app/components/FPC/FPC";
import { inject, observer } from "mobx-react";
import * as React from 'react';
import { Link, Redirect, Route, Switch } from "react-router-dom";
import "./App.scss";
import { AppProps } from './AppProps';
import { withRouter } from "react-router";

@inject("autopeeringStore")
@observer
class App extends React.Component<AppProps, any> {
    componentDidMount(): void {
        this.props.autopeeringStore.connect();
    }

    render() {
        return (
            <div className="root">
                <header>
                    <Link className="brand" to="/">
                        <img src="/assets/logo-header.svg" alt="GoShimmer Analyser" />
                        <h1>GoShimmer Analyzer</h1>
                    </Link>
                    <nav>
                        <Link to="/autopeering">
                            Autopeering
                        </Link>
                        <Link to="/consensus">
                            Consensus
                        </Link>
                    </nav>
                    <div className="badge-container">
                        {!this.props.autopeeringStore.websocketConnected &&
                            <div className="badge">Not connected</div>
                        }
                    </div>
                </header>
                <Switch>
                    <Route path="/autopeering" component={Autopeering} />
                    <Route exact path="/consensus" component={FPC} />
                    <Route path="/consensus/conflict/:id" component={Conflict} />
                    <Redirect to="/autopeering" />
                </Switch>
                {this.props.children}
            </div>
        );
    }
}

export default withRouter(App);