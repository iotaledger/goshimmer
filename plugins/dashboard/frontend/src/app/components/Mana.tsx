import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import Col from "react-bootstrap/Col";
import ManaChart from "app/components/ManaChart";

interface Props {
    nodeStore?: NodeStore;
}

var customData = [
    [new Date(2020,0,0,0,0,0,0), 1500, 2000],
    [new Date(2020,0,0,0,1,0,0), 1800, 2325],
    [new Date(2020,0,0,0,2,0,0), 1900, 2896],
    [new Date(2020,0,0,0,3,0,0), 2052, 1752],
    [new Date(2020,0,0,0,4,0,0), 2456, 1968],
    [new Date(2020,0,0,0,5,0,0), 986, 1100],
    [new Date(2020,0,0,0,6,0,0), 236, 1056],
    [new Date(2020,0,0,0,7,0,0), 500, 785],
    [new Date(2020,0,0,0,8,0,0), 1300, 1800],
]


@inject("nodeStore")
@observer
export class Mana extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Mana</h3>
                <Row className={"mb-3"}>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaChart
                            node={this.props.nodeStore.status.id}
                            data={customData}
                        />
                    </Col>
                </Row>
            </Container>
        );
    }
}
