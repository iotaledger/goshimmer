import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import {resolveBase58BranchID} from "app/utils/branch";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    txId: string;
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerTransactionMetadata extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getTransactionMetadata(this.props.txId);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let { txId } = this.props;
        let { query_err, txMetadata } = this.props.explorerStore;

        if (query_err) {
            return (
                <Container>
                <h4>Metadata</h4>
                    <p> Metadata for transaction ID {txId} not found.</p>
                </Container>
            );
        }
        return (
            <Container>
                <h4>Metadata</h4>
                {txMetadata && <ListGroup>
                    <ListGroup.Item>Branch ID: <a href={`/explorer/branch/${txMetadata.branchID}`}>{resolveBase58BranchID(txMetadata.branchID)}</a></ListGroup.Item>
                    <ListGroup.Item>Solid: {txMetadata.solid.toString()}</ListGroup.Item>
                    <ListGroup.Item>Solidification time: {new Date(txMetadata.solidificationTime * 1000).toLocaleString()}</ListGroup.Item>
                    <ListGroup.Item>Finalized: {txMetadata.finalized.toString()}</ListGroup.Item>
                    <ListGroup.Item>Lazy booked: {txMetadata.lazyBooked.toString()}</ListGroup.Item>
                </ListGroup>}
            </Container>
        )
    }
}