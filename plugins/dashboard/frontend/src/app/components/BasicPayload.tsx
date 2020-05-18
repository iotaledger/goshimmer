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
export class BasicPayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;
        return (
            payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        {payload.content_title}: {' '} 
                        {payload.content}
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
