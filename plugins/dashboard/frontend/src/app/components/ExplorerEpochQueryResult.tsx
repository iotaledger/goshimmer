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
export class ExplorerEpochQueryResult extends React.Component<Props, any> {
    componentDidMount() {
        const id = this.props.match.params.commitment;
        this.props.explorerStore.getEpochDetails(id);

        const index = Number(id.split(':')[1]);
        this.props.explorerStore.getEpochBlocks(index);
        this.props.explorerStore.getEpochTransactions(index);
        this.props.explorerStore.getEpochUTXOs(index);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }

    render() {
        let {commitment} = this.props.match.params;
        let { query_err, epochInfo,  epochBlocks, epochTransactions, epochUtxos } = this.props.explorerStore;

        if (query_err) {
            return (
                <Container>
                    <h4>Epoch not found - 404</h4>
                    <span>{commitment}</span>
                </Container>
            );
        }
        return (
            <Container>
                <h4>Epoch</h4>
                {epochInfo && <ListGroup>
                    <ListGroup.Item>ID: {commitment}</ListGroup.Item>
                    <ListGroup.Item>Index: {epochInfo.index}</ListGroup.Item>
                    <ListGroup.Item>RootsID: {epochInfo.rootsID}</ListGroup.Item>
                    <ListGroup.Item>PrevEC: {epochInfo.prevID}</ListGroup.Item>
                    <ListGroup.Item>Cumulative Weight: {epochInfo.cumulativeWeight}</ListGroup.Item>
                     <ListGroup.Item>Blocks:
                        {epochBlocks.blocks && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {epochBlocks.blocks.map((b,i) => <ListGroup.Item key={i}><a href={`/explorer/block/${b}`}>{b}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                    <ListGroup.Item>Transactions:
                        {epochTransactions.transactions && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {epochTransactions.transactions.map((t,i) => <ListGroup.Item key={i}><a href={`/explorer/transaction/${t}`}>{t}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                    <ListGroup.Item> Created outputs:
                        {epochUtxos.createdOutputs && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {epochUtxos.createdOutputs.map((c,i) => <ListGroup.Item key={i}><a href={`/explorer/output/${c}`}>{c}</a></ListGroup.Item>)}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                        </Card>}
                    </ListGroup.Item>
                    <ListGroup.Item> Spent outputs:
                        {epochUtxos.spentOutputs && <Card>
                        <Card.Body>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'max-height':'300px', 'overflow':'auto'}}>
                                    <Table>
                                        <tbody>
                                        {epochUtxos.spentOutputs.map((s,i) => <ListGroup.Item key={i}><a href={`/explorer/output/${s}`}>{s}</a></ListGroup.Item>)}
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
