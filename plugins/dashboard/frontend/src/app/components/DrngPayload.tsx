import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import {DrngSubtype} from "app/misc/Payload"

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class DrngPayload extends React.Component<Props, any> {

    render() {
        let {payload, subpayload} = this.props.explorerStore;
        return (
            payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Drng payload subtype: {payload.subpayload_type} </ListGroup.Item> 
                        </ListGroup>
                    </Col>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Instance ID: {payload.instance_id} </ListGroup.Item> 
                        </ListGroup>
                    </Col>
                </Row>
                { 
                    payload.subpayload_type == DrngSubtype.Cb ? (   
                        <React.Fragment>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item>Round: {subpayload.round} </ListGroup.Item> 
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item>Previous Signature: {subpayload.prev_sig} </ListGroup.Item> 
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item>Signature: {subpayload.sig} </ListGroup.Item> 
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item>Distributed Public Key: {subpayload.dpk} </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                            </Row>
                        </React.Fragment>
                    ) : (
                        <React.Fragment>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        <ListGroup.Item>{subpayload.content_title}: {subpayload.bytes} </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                        </Row>
                        </React.Fragment>
                    )
                }
            </React.Fragment>
        );
    }
}
