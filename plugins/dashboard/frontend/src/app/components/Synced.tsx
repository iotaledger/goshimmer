import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Synced extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Synced: {this.props.nodeStore.status.synced? "Yes":"No"}
            </React.Fragment>
        );
    }
}
