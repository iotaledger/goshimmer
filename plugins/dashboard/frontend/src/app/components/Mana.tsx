import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ManaChart from "app/components/ManaChart";
import RichestMana from "app/components/ManaRichest";
import ManaHistogram from "app/components/ManaHistogram";
import {Col} from "react-bootstrap";
import {ManaStore} from "app/stores/ManaStore";

interface Props {
    nodeStore?: NodeStore;
    manaStore?: ManaStore;
}

// var customData = [
//     [new Date(2020,0,0,0,0,0,0), 1500, 2000],
//     [new Date(2020,0,0,0,1,0,0), 1800, 2325],
//     [new Date(2020,0,0,0,2,0,0), 1900, 2896],
//     [new Date(2020,0,0,0,3,0,0), 2052, 1752],
//     [new Date(2020,0,0,0,4,0,0), 2456, 1968],
//     [new Date(2020,0,0,0,5,0,0), 986, 1100],
//     [new Date(2020,0,0,0,6,0,0), 236, 1056],
//     [new Date(2020,0,0,0,7,0,0), 500, 785],
//     [new Date(2020,0,0,0,8,0,0), 1300, 1800],
// ]

@inject("nodeStore")
@inject("manaStore")
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
                            data={this.props.manaStore.manaValues}
                        />
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaHistogram data={this.props.manaStore.accessHistogramInput}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <RichestMana data={this.props.manaStore.networkRichestFeedAccess} title={"Richest Access Mana Nodes"}/>
                    </Col>
                    <Col>
                        <RichestMana data={this.props.manaStore.networkRichestFeedConsensus} title={"Richest Consensus Mana Nodes"}/>
                    </Col>
                </Row>
                <Row>
                    <Col>
                        <RichestMana data={this.props.manaStore.onlineRichestFeedAccess} title={"Online Richest Access Mana Nodes"}/>
                    </Col>
                    <Col>
                        <RichestMana data={this.props.manaStore.onlineRichestFeedConsensus} title={"Online Richest Consensus Mana Nodes"}/>
                    </Col>
                </Row>
            </Container>
        );
    }
}
