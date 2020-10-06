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
import ManaGauge from "app/components/ManaGauge";
import ManaAllowedPledgeID from "app/components/ManaAllowedPledgeID";
import ManaPercentile from "app/components/ManaPercentile";
import ManaEventList from "app/components/ManaEventList";

interface Props {
    nodeStore?: NodeStore;
    manaStore?: ManaStore;
}

@inject("nodeStore")
@inject("manaStore")
@observer
export class Mana extends React.Component<Props, any> {
    render() {
        let manaStore = this.props.manaStore;
        let nodeStore = this.props.nodeStore;
        const {searchNode} = this.props.manaStore;
        return (
            <Container>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaGauge
                            data={manaStore.manaValues.length === 0 ? [0.0, 0.0] :[manaStore.manaValues[manaStore.manaValues.length -1][1], manaStore.prevManaValues[0]]}
                            title={"Access Mana"}
                        />
                    </Col>
                    <Col>
                        <ManaGauge
                            data={manaStore.manaValues.length === 0 ? [0.0, 0.0] :[manaStore.manaValues[manaStore.manaValues.length -1][2], manaStore.prevManaValues[1]]}
                            title={"Consensus Mana"}
                        />
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>
                                    <Container fluid style={{padding: '0rem'}}>
                                        <Row>
                                            <Col>
                                                Access Percentile
                                            </Col>
                                            <Col>
                                                <b>{manaStore.accessPercentile.toFixed(2)} %</b>
                                            </Col>
                                        </Row>
                                    </Container>
                                </Card.Title>
                                <ManaPercentile data={manaStore.accessPercentile} key={manaStore.accessPercentile.toString()}/>
                            </Card.Body>
                        </Card>
                    </Col>
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>
                                    <Container fluid style={{padding: '0rem'}}>
                                        <Row>
                                            <Col>
                                                Consensus Percentile
                                            </Col>
                                            <Col>
                                                <b>{manaStore.consensusPercentile.toFixed(2)} %</b>
                                            </Col>
                                        </Row>
                                    </Container>
                                </Card.Title>
                                <ManaPercentile data={manaStore.consensusPercentile} key={manaStore.consensusPercentile.toString()}/>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <ManaAllowedPledgeID manaStore={manaStore}/>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaChart
                            node={nodeStore.status.id}
                            data= {manaStore.manaValues.length === 0 ? [[new Date(), 0.0, 0.0]] :manaStore.manaValues}
                        />
                    </Col>
                </Row>
                <Row>
                    <Col>
                        <ManaEventList
                            title={"Access Events"}
                            listItems={manaStore.accessEventList}
                        />
                    </Col>
                    <Col>
                        <ManaEventList
                            title={"Consensus Events"}
                            listItems={manaStore.consensusEventList}
                        />
                    </Col>
                </Row>
                <div className="mb-3">
                    <label>
                        Search Node
                    </label>
                    <input
                        placeholder="Enter a node id"
                        type="text"
                        value={searchNode}
                        onChange={(e) => manaStore.updateSearch(e.target.value)}
                    />
                </div>
                <Row className={"mb-3"}>
                    <Col>
                        <RichestMana data={manaStore.networkRichestFeedAccess} title={"Access Mana Leaderboard"}/>
                    </Col>
                    <Col>
                        <RichestMana data={manaStore.networkRichestFeedConsensus} title={"Consensus Mana Leaderboard"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <RichestMana data={manaStore.activeRichestFeedAccess} title={"Active Access Mana Leaderboard"}/>
                    </Col>
                    <Col>
                        <RichestMana data={manaStore.activeRichestFeedConsensus} title={"Active Consensus Mana Leaderboard"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaHistogram data={manaStore.accessHistogramInput} title={"Access Mana Distribution"}/>
                    </Col>
                    <Col>
                        <ManaHistogram data={manaStore.consensusHistogramInput} title={"Consensus Mana Distribution"}/>
                    </Col>
                </Row>
            </Container>
        );
    }
}
