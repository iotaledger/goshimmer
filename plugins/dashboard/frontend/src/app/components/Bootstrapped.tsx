import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Bootstrapped extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Bootstrapped: {this.props.nodeStore.status.tangleTime.bootstrapped ? "Yes" : "No"}
            </React.Fragment>
        );
    }
}
