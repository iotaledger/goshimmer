import { Autopeering } from "app/components/Autopeering/Autopeering";
import Conflict from "app/components/FPC/Conflict";
import FPC from "app/components/FPC/FPC";
import { inject, observer } from "mobx-react";
import * as React from 'react';
import { Redirect, Route, Switch, Link } from "react-router-dom";
import "./Root.scss";
import logoHeader from "../../../assets/logo-header.svg";
import { RootProps } from './RootProps';

@inject("routerStore")
@inject("autopeeringStore")
@observer
export class Root extends React.Component<RootProps, any> {
    componentDidMount(): void {
        this.props.autopeeringStore.connect();
    }

    render() {
        return (
            <div className="root">
                <header>
                    <Link className="brand" to="/">
                        <img src={logoHeader} alt="GoShimmer Analyser" />
                        <h1>GoShimmer Analyzer</h1>
                    </Link>
                    <nav>
                        <Link to="/autopeering">
                            Autopeering
                        </Link>
                        <Link to="/fpc-example">
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
                    <Route exact path="/autopeering" component={Autopeering} />
                    <Route exact path="/fpc-example" component={FPC} />
                    <Route exact path="/fpc-example/conflict/:id" component={Conflict} />
                    <Redirect to="/autopeering" />
                </Switch>
                {this.props.children}
            </div>
        );
    }
}
