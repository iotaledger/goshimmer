import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import {Link} from 'react-router-dom';
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
            <div style={{marginTop: "20px", marginBottom: "20px"}}>
                <h4>Metadata</h4>
                {txMetadata && <ListGroup>
                    BranchIDs: 
                    <ListGroup>
                        {
                            txMetadata.branchIDs.map((value, index) => {
                                return (
                                    <ListGroup.Item key={"BranchID" + index + 1} className="text-break">
                                        <Link to={`/explorer/branch/${value}`}>
                                            {resolveBase58BranchID(value)}
                                        </Link>
                                    </ListGroup.Item>
                                )
                            })
                        }
                    </ListGroup>
                    <ListGroup.Item>Solid: {txMetadata.solid.toString()}</ListGroup.Item>
                    <ListGroup.Item>Solidification time: {new Date(txMetadata.solidificationTime * 1000).toLocaleString()}</ListGroup.Item>
                    <ListGroup.Item>Grade of Finality: {txMetadata.gradeOfFinality}</ListGroup.Item>
                    <ListGroup.Item>Grade of Finality Time: {new Date(txMetadata.gradeOfFinalityTime * 1000).toLocaleString()}</ListGroup.Item>
                    <ListGroup.Item>Lazy booked: {txMetadata.lazyBooked.toString()}</ListGroup.Item>
                </ListGroup>}
            </div>
        )
    }
}