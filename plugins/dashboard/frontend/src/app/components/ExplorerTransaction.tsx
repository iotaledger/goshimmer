import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore, { GenesisTransactionID } from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import * as dateformat from 'dateformat';

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    txId: string
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerTransaction extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getTransaction(this.props.txId);
    }
    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let { txId } = this.props;
        let { query_err, tx } = this.props.explorerStore;

        if (txId === GenesisTransactionID) {
            return (
                <Container>
                    <h3>Genesis Transaction ID</h3>
                    <p>This represents the identifier of the genesis Transaction.</p>
                </Container>
            )
        }
        if (query_err) {
            return (
                <Container>
                    <h3>Transaction not available - 404</h3>
                    <p>
                        Transaction with ID {txId} not found.
                    </p>
                </Container>
            );
        }
        return (
            <Container>
                <h3>Transaction</h3>
                <p> {txId} </p>


                {tx &&
                    <Row className={"mb-3"}>
                        <Col>
                            <ListGroup>
                                <ListGroup.Item>ID: {txId}</ListGroup.Item>
                                <ListGroup.Item>Version: {tx.version}</ListGroup.Item>
                                <ListGroup.Item>Timestamp: {dateformat(new Date(tx.timestamp * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                                <ListGroup.Item>Access pledge ID: {tx.accessPledgeID}</ListGroup.Item>
                                <ListGroup.Item>Consensus pledge ID: {tx.consensusPledgeID}</ListGroup.Item>
                                <ListGroup.Item>
                                    <span className={"mb-2"}>Inputs</span>
                                        {tx.inputs.map((input, i) => {
                                            return (
                                                <ListGroup key={i} className={"mb-1"}>
                                                    <ListGroup.Item>Input Type: {input.type}</ListGroup.Item>
                                                    <ListGroup.Item>Referenced OutputID: <a href={"#"}>{input.referencedOutputID.id}</a></ListGroup.Item>
                                                    <ListGroup.Item>OutputIndex: {input.referencedOutputID.outputIndex}</ListGroup.Item>
                                                    <ListGroup.Item>Transaction ID: <a href={`/explorer/transaction/${input.referencedOutputID.transactionID}`}>{input.referencedOutputID.transactionID}</a></ListGroup.Item>
                                                </ListGroup>
                                            )
                                        })}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    <span className={"mb-2"}> Outputs</span>
                                    {tx.outputs.map((output, i) => {
                                        return <ListGroup.Item key={i}><a href={"#"}>{output.outputID.id}</a></ListGroup.Item>
                                    })}
                                </ListGroup.Item>
                                <ListGroup.Item>Data payload: {tx.dataPayload}</ListGroup.Item>
                            </ListGroup>
                        </Col>
                    </Row>
                }
            </Container>
        )
    }
}