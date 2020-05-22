import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {FPCStore} from "app/stores/FPCStore";
import Row from "react-bootstrap/Row";
import Container from "react-bootstrap/Container";
import Card from "react-bootstrap/Card";

interface Props {
    nodeStore?: NodeStore;
    fpcStore?: FPCStore;
    match?: {
        params: {
            id: string,
        }
    }
}

@inject("nodeStore")
@inject("fpcStore")
@observer
export default class Conflict extends React.Component<Props, any> {
    componentDidMount() {
        this.props.fpcStore.updateCurrentConflict(this.props.match.params.id);
    }

    render() {
        let {nodeGrid} = this.props.fpcStore;
        return (
            <Container>
                <h3>Conflict detail</h3>
                <Card>
                    <Card.Body>
                        <Card.Title>ID: {this.props.match.params.id}</Card.Title>
                        <Row>
                            {nodeGrid}
                        </Row>
                    </Card.Body>
                </Card>
            </Container>
        );
    }
}
