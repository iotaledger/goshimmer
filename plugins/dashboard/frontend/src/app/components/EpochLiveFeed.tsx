import { EpochStore } from 'app/stores/EpochStore';
import {inject, observer} from "mobx-react";
import NodeStore from 'app/stores/NodeStore';
import * as React from 'react';
import Container from "react-bootstrap/Container";
import { Table } from 'react-bootstrap';

interface Props {
    history: any;
    nodeStore?: NodeStore;
    epochStore?: EpochStore;
}

@inject("nodeStore")
@inject("epochStore")
@observer
export class EpochLiveFeed extends React.Component<Props, any> {
    render() {
        let {epochLiveFeed} = this.props.epochStore;
        return (
            <Container>
                <h3>Epochs</h3>
                <Table bordered>
                    <thead>
                    <tr>
                        <th>Index</th>
                        <th>Commitment ID</th>
                    </tr>
                    </thead>
                    <tbody>
                    {epochLiveFeed}
                    </tbody>
                </Table>
            </Container>
        );
    }
}
