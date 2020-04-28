import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Version extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Version {this.props.nodeStore.status.version}
            </React.Fragment>
        );
    }
}
