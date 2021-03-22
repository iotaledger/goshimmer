import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore, { GenesisTransactionID } from "app/stores/ExplorerStore";
import * as dateformat from 'dateformat';

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

        if (txId === GenesisTransactionID) {
            return (
                <Container>
                <h4>Metadata</h4>
                    <p>No metadata for genesis transaction</p>
                </Container>
            )
        }
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
                {txMetadata && <Row className={"mb-3"}>
                    <Col>
                        <table className={"table table-bordered table-condensed table-sm"}>
                            <tbody>
                                <tr>
                                    <td>Branch ID</td>
                                    <td><a href="#">{txMetadata.branchID}</a></td>
                                </tr>
                                <tr>
                                    <td>Solid</td>
                                    <td>{txMetadata.solid.toString()}</td>
                                </tr>
                                <tr>
                                    <td>Solidification time</td>
                                    <td>{dateformat(new Date(txMetadata.solidificationTime * 1000), "dd.mm.yyyy HH:MM:ss")}</td>
                                </tr>
                                <tr>
                                    <td>Finalized</td>
                                    <td>{txMetadata.finalized.toString()}</td>
                                </tr>
                                <tr>
                                    <td>Lazy booked</td>
                                    <td>{txMetadata.lazyBooked.toString()}</td>
                                </tr>
                            </tbody>
                        </table>
                    </Col>
                </Row>}
            </Container>
        )
    }
}