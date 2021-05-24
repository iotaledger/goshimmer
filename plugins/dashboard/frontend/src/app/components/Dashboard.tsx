import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Uptime from "app/components/Uptime";
import Version from "app/components/Version";
import MPSChart from "app/components/MPSChart";
import TipsChart from "app/components/TipsChart";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ListGroup from "react-bootstrap/ListGroup";
import Card from "react-bootstrap/Card";
import MemChart from "app/components/MemChart";
import ComponentCounterChart from "app/components/ComponentCounterChart";
import TangleTime from "app/components/TangleTime";
interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Dashboard extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Dashboard</h3>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>Node: {this.props.nodeStore.status.id}</Card.Title>
                                <Row>
                                    <Col>
                                        <ListGroup variant={"flush"}>
                                            <ListGroup.Item><Uptime/></ListGroup.Item>
                                        </ListGroup>
                                    </Col>
                                    <Col>
                                        <ListGroup variant={"flush"}>
                                            <ListGroup.Item><Version/></ListGroup.Item>
                                        </ListGroup>
                                    </Col>
                                </Row>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><TangleTime/></Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><MPSChart/></Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><TipsChart/></Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><ComponentCounterChart/></Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col><MemChart/></Col>
                </Row>
            </Container>
        );
    }
}
