import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import {Link} from 'react-router-dom';
import ListGroup from "react-bootstrap/ListGroup";
import {resolveBase58ConflictID} from "app/utils/conflict";
import {resolveConfirmationState} from "app/utils/confirmation_state";

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
            <div style={{marginTop: "20px", marginBottom: "20px"}}>
                <h4>Metadata</h4>
                {txMetadata && <ListGroup>
                    ConflictIDs: 
                    <ListGroup>
                        {
                            txMetadata.conflictIDs.map((value, index) => {
                                return (
                                    <ListGroup.Item key={"ConflictID" + index + 1} className="text-break">
                                        <Link to={`/explorer/conflict/${value}`}>
                                            {resolveBase58ConflictID(value)}
                                        </Link>
                                    </ListGroup.Item>
                                )
                            })
                        }
                    </ListGroup>
                    <ListGroup.Item>Booked: {txMetadata.booked.toString()}</ListGroup.Item>
                    <ListGroup.Item>Booked time: {new Date(txMetadata.bookedTime * 1000).toLocaleString()}</ListGroup.Item>
                    <ListGroup.Item>Confirmation State: {resolveConfirmationState(txMetadata.confirmationState)}</ListGroup.Item>
                    <ListGroup.Item>Confirmation State Time: {new Date(txMetadata.confirmationStateTime * 1000).toLocaleString()}</ListGroup.Item>
                </ListGroup>}
            </div>
        )
    }
}
