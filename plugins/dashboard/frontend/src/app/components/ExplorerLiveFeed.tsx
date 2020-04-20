import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Card from "react-bootstrap/Card";
import ExplorerStore from "app/stores/ExplorerStore";
import Table from "react-bootstrap/Table";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerLiveFeed extends React.Component<Props, any> {
    render() {
        let {txsLiveFeed} = this.props.explorerStore;
        return (
            <Row className={"mb-3"}>
                <Col>
                    <Card>
                        <Card.Body>
                            <Card.Title>Live Feed</Card.Title>
                            <Row className={"mb-3"}>
                                <Col xs={12}>
                                    <h6>Transactions</h6>
                                    <Table>
                                        <thead>
                                        <tr>
                                            <td>Hash</td>
                                            <td>Value</td>
                                        </tr>
                                        </thead>
                                        <tbody>
                                        {txsLiveFeed}
                                        </tbody>
                                    </Table>
                                </Col>
                            </Row>
                        </Card.Body>
                    </Card>
                </Col>
            </Row>
        );
    }
}
