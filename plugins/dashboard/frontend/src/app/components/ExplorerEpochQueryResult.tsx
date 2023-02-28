import * as React from 'react';
import Container from "react-bootstrap/Container";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import { Card, Col, Row, Table } from 'react-bootstrap';


interface Props {
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            commitment: string,
        }
    }
}

@inject("explorerStore")
@observer
export class ExplorerSlotQueryResult extends React.Component<Props, any> {
    componentDidMount() {
        const id = this.props.match.params.commitment;
        this.props.explorerStore.getSlotDetails(id);

        const index = Number(id.split(':')[1]);
        this.props.explorerStore.getSlotBlocks(index);
        this.props.explorerStore.getSlotTransactions(index);
        this.props.explorerStore.getSlotUTXOs(index);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }

    render() {
        let {commitment} = this.props.match.params;
        let { query_err, slotInfo,  slotBlocks, slotTransactions, slotUtxos } = this.props.explorerStore;

        if (query_err) {
            return (
                <Container>
                    <h4>Slot not found - 404</h4>
                    <span>{commitment}</span>
                </Container>
            );
        }
        return (
            <Container>
                <h4>Slot</h4>
                {slotInfo && <ListGroup>
                    <ListGroup.Item>ID: {commitment}</ListGroup.Item>
                    <ListGroup.Item>Index: {slotInfo.index}</ListGroup.Item>
                    <ListGroup.Item>RootsID: {slotInfo.rootsID}</ListGroup.Item>
                    <ListGroup.Item>PrevEC: {slotInfo.prevID}</ListGroup.Item>
                    <ListGroup.Item>Cumulative Weight: {slotInfo.cumulativeWeight}</ListGroup.Item>
                     <ListGroup.Item>Blocks:
                        {slotBlocks.blocks && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {slotBlocks.blocks.map((b,i) => <ListGroup.Item key={i}><a href={`/explorer/block/${b}`}>{b}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                    <ListGroup.Item>Transactions:
                        {slotTransactions.transactions && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {slotTransactions.transactions.map((t,i) => <ListGroup.Item key={i}><a href={`/explorer/transaction/${t}`}>{t}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                    <ListGroup.Item> Created outputs:
                        {slotUtxos.createdOutputs && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {slotUtxos.createdOutputs.map((c,i) => <ListGroup.Item key={i}><a href={`/explorer/output/${c}`}>{c}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                    <ListGroup.Item> Spent outputs:
                        {slotUtxos.spentOutputs && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {slotUtxos.spentOutputs.map((s,i) => <ListGroup.Item key={i}><a href={`/explorer/output/${s}`}>{s}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                </ListGroup>}
            </Container>
        )
    }
}
