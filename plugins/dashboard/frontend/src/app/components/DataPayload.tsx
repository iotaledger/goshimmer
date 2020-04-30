import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class DataPayload extends React.Component<Props, any> {

    render() {
        let {msg} = this.props.explorerStore;
        return (
            msg.payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        {msg.payload}
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
