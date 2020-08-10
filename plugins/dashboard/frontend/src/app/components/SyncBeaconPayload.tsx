import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import * as dateformat from 'dateformat';

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class SyncBeaconPayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;
        return (
            payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Sent Time: {dateformat(new Date(payload.sent_time/1000000), "dd.mm.yyyy HH:MM:ss")} </ListGroup.Item> 
                        </ListGroup>
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
