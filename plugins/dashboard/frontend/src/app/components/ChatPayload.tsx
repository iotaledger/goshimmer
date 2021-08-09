import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class ChatPayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;
        return (
            payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>From: {payload.from} </ListGroup.Item> 
                        </ListGroup>
                    </Col>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>To: {payload.to} </ListGroup.Item> 
                        </ListGroup>
                    </Col>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Message: {payload.message} </ListGroup.Item> 
                        </ListGroup>
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
