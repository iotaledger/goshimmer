import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import {ListGroupItem} from "react-bootstrap";

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class StatementPayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;
        return (
            payload &&
            <React.Fragment>
                {
                    payload.conflicts &&
                    <Row className={"mb-3"}>
                        <Col>
                            Conflicts
                            <ListGroup>
                                {
                                    payload.conflicts.map( (value) => {
                                        return (
                                            <ListGroupItem>

                                                Transaction ID: {value.tx_id}
                                                <ul>
                                                    <li>Opinion: {value.opinion.value}</li>
                                                    <li>Round: {value.opinion.round}</li>
                                                </ul>
                                            </ListGroupItem>
                                        )
                                    })}
                            </ListGroup>
                        </Col>
                    </Row>
                }
                {
                    payload.timestamps &&
                    <Row className={"mb-3"}>
                        <Col>
                            Timestamps
                            <ListGroup>
                                {payload.timestamps.map( (value) => {
                                    return (
                                        <ListGroupItem>
                                            Message ID: {value.msg_id}
                                            <ul>
                                                <li>Opinion: {value.opinion.value}</li>
                                                <li>Round: {value.opinion.round}</li>
                                            </ul>
                                        </ListGroupItem>
                                    )
                                })}
                            </ListGroup>
                        </Col>
                    </Row>
                }

            </React.Fragment>
        );
    }
}
