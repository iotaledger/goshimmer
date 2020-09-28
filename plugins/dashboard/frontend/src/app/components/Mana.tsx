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
                        <RichestMana data={this.props.manaStore.networkRichestFeedAccess} title={"Richest Access Mana Nodes"}/>
                    </Col>
                    <Col>
                        <RichestMana data={this.props.manaStore.networkRichestFeedConsensus} title={"Richest Consensus Mana Nodes"}/>
                    </Col>
                </Row>
                <Row>
                    <Col>
                        <RichestMana data={this.props.manaStore.activeRichestFeedAccess} title={"Online Richest Access Mana Nodes"}/>
                    </Col>
                    <Col>
                        <RichestMana data={this.props.manaStore.activeRichestFeedConsensus} title={"Online Richest Consensus Mana Nodes"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaHistogram data={this.props.manaStore.accessHistogramInput}/>
                    </Col>
                </Row>
            </Container>
        );
    }
}
