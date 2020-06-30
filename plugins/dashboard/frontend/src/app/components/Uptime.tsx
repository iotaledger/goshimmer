import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Uptime extends React.Component<Props, any> {
    render() {
        return (
            <React.Fragment>
                Uptime {this.props.nodeStore.uptime}
            </React.Fragment>
        );
    }
}
