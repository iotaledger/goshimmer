import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Card from "react-bootstrap/Card";
import ChatStore from "app/stores/ChatStore";
import Table from "react-bootstrap/Table";

interface Props {
    nodeStore?: NodeStore;
    chatStore?: ChatStore;
}

@inject("nodeStore")
@inject("chatStore")
@observer
export class ChatLiveFeed extends React.Component<Props, any> {
    render() {
        let {msgsLiveFeed} = this.props.chatStore;
        return (
            <Row className={"mb-3"}>
                <Col>
                    <Card>
                        <Card.Body>
                            <Card.Title>Live Feed</Card.Title>
                            <Row className={"mb-3"}>
                                <Col xs={12} style={{'height':'500px', 'overflow':'auto'}}>
                                    <h6>Messages</h6>
                                    <Table>
                                        <thead>
                                        <tr>
                                            <td>From</td>
                                            <td>Message</td>
                                            <td>MessageID</td>
                                            <td>Timestamp</td>
                                        </tr>
                                        </thead>
                                        <tbody >
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
