import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {FPCStore} from "app/stores/FPCStore";
import Row from "react-bootstrap/Row";
import Container from "react-bootstrap/Container";

interface Props {
    nodeStore?: NodeStore;
    fpcStore?: FPCStore;
}

@inject("nodeStore")
@inject("fpcStore")
@observer
export default class FPC extends React.Component<Props, any> {
    render() {
        let {nodeGrid} = this.props.fpcStore;
        return (
            <Container>
                <Row>
                    {nodeGrid}
                </Row>
            </Container>
        );
    }
}
