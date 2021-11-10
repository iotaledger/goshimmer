import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ConflictsStore from "app/stores/ConflictsStore";
import Table from "react-bootstrap/Table";

interface Props {
    nodeStore?: NodeStore;
    conflictsStore?: ConflictsStore;
}

@inject("nodeStore")
@inject("conflictsStore")
@observer
export class Conflicts extends React.Component<Props, any> {
    render() {
        let {conflictsLiveFeed} = this.props.conflictsStore;
        return (
            <Container>
                <h3>Conflicts</h3>
                <Table bordered>
                    <thead>
                    <tr>
                        <th>ConflictID</th>
                        <th>ArrivalTime</th>
                        <th>Resolved</th>
                        <th>TimeToResolve - ms</th>
                    </tr>
                    </thead>
                    <tbody>
                    {conflictsLiveFeed}
                    </tbody>
                </Table>
            </Container>
        );
    }
}
