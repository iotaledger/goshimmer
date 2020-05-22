import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Uptime from "app/components/Uptime";
import Version from "app/components/Version";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ListGroup from "react-bootstrap/ListGroup";
import Card from "react-bootstrap/Card";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Autopeering extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Autopeering</h3>
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
            </Container>
        );
    }
}
