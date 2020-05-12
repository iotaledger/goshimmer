import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {FPCStore, LightenDarkenColor, Node} from "app/stores/FPCStore";
import Row from "react-bootstrap/Row";
import Container from "react-bootstrap/Container";
import Col from "react-bootstrap/Col";

interface Props {
    nodeStore?: NodeStore;
    fpcStore?: FPCStore;
}

@inject("nodeStore")
@inject("fpcStore")
@observer
export default class FPC extends React.Component<Props, any> {
    render() {
        let {nodes} = this.props.fpcStore;
        let nodeSquares = [];
        nodes.forEach((node: Node, id: number, obj: Map<number, Node>) => {
            nodeSquares.push(
                <Col xs={1} key={id.toString()} style={{
                    height: 50,
                    width: 50,
                    background: LightenDarkenColor("#707070", node.opinion),
                }}>
                    {node.opinion}
                </Col>
            )
        })
        return (
            <Container>
                <Row>
                    {nodeSquares}
                </Row>
            </Container>
        );
    }
}
