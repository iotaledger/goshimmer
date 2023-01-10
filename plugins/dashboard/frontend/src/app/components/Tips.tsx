import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Table from "react-bootstrap/Table";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class Tips extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getTips();
    }
    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let {tipsList} = this.props.explorerStore;
        return (
            <Container>
                <h3>Tips</h3>
                <Table bordered>
                    <thead>
                    <tr>
                        <th>BlockID</th>
                    </tr>
                    </thead>
                    <tbody>
                    {tipsList}
                    </tbody>
                </Table>
            </Container>
        );
    }
}