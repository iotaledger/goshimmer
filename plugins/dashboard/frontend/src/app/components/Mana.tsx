import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ManaChart from "app/components/ManaChart";
import RichestMana from "app/components/ManaRichest";
import ManaHistogram from "app/components/ManaHistogram";
import {Card, Col} from "react-bootstrap";
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
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>
                                    Access Mana Percentile
                                </Card.Title>
                                {this.props.manaStore.accessPercentile}
                            </Card.Body>
                        </Card>
                    </Col>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>
                                    Consensus Mana Percentile
                                </Card.Title>
                                {this.props.manaStore.consensusPercentile}
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaChart
                            node={this.props.nodeStore.status.id}
                            data= {this.props.manaStore.manaValues.length === 0 ? [[new Date(), 0.0, 0.0]] :this.props.manaStore.manaValues}
                        />
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <RichestMana data={this.props.manaStore.networkRichestFeedAccess} title={"Access Mana Leaderborad"}/>
                    </Col>
                    <Col>
                        <RichestMana data={this.props.manaStore.networkRichestFeedConsensus} title={"Consensus Mana Leaderboard"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <RichestMana data={this.props.manaStore.activeRichestFeedAccess} title={"Active Access Mana Leaderboard"}/>
                    </Col>
                    <Col>
                        <RichestMana data={this.props.manaStore.activeRichestFeedConsensus} title={"Active Consensus Mana Leaderboard"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaHistogram data={this.props.manaStore.accessHistogramInput} title={"Access Mana Distribution"}/>
                    </Col>
                    <Col>
                        <ManaHistogram data={this.props.manaStore.consensusHistogramInput} title={"Consensus Mana Distribution"}/>
                    </Col>
                </Row>
            </Container>
        );
    }
}
