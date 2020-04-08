import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Card from "react-bootstrap/Card";
import DrngStore from "app/stores/DrngStore";
import Table from "react-bootstrap/Table";

interface Props {
    nodeStore?: NodeStore;
    drngStore?: DrngStore;
}

@inject("nodeStore")
@inject("drngStore")
@observer
export class DrngLiveFeed extends React.Component<Props, any> {
    render() {
        let {msgsLiveFeed} = this.props.drngStore;
        return (
            <Row className={"mb-3"}>
                <Col>
                    <Card>
                        <Card.Body>
                            <Card.Title>Live Feed</Card.Title>
                            <Row className={"mb-3"}>
                                <Col xs={12}>
                                    <h6>Messages</h6>
                                    <Table>
                                        <thead>
                                        <tr>
                                            <td>ID</td>
                                            <td>Random value</td>
                                        </tr>
                                        </thead>
                                        <tbody>
                                        {msgsLiveFeed}
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
