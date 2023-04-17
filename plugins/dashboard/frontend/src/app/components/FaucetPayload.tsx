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
export class FaucetPayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;
        return (
            payload &&
            <React.Fragment>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Address: {payload.address}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Access Mana Pledge ID: {payload.accessManaPledgeID}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Consensus Mana Pledge ID: {payload.consensusManaPledgeID}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Nonce: {payload.nonce}
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>
            </React.Fragment>
        );
    }
}
