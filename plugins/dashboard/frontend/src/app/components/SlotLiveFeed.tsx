import { SlotStore } from 'app/stores/SlotStore';
import {inject, observer} from "mobx-react";
import NodeStore from 'app/stores/NodeStore';
import * as React from 'react';
import Container from "react-bootstrap/Container";
import { Table } from 'react-bootstrap';

interface Props {
    history: any;
    nodeStore?: NodeStore;
    slotStore?: SlotStore;
}

@inject("nodeStore")
@inject("slotStore")
@observer
export class SlotLiveFeed extends React.Component<Props, any> {
    render() {
        let {slotLiveFeed} = this.props.slotStore;
        return (
            <Container>
                <h3>Slots</h3>
                <Table bordered>
                    <thead>
                    <tr>
                        <th>Index</th>
                        <th>Commitment ID</th>
                    </tr>
                    </thead>
                    <tbody>
                    {slotLiveFeed}
                    </tbody>
                </Table>
            </Container>
        );
    }
}
