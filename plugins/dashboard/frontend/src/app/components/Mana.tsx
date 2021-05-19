import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ManaChart from "app/components/ManaChart";
import ManaLeaderboard from "app/components/ManaLeaderboard";
import ManaHistogram from "app/components/ManaHistogram";
import {Badge, Card, Col} from "react-bootstrap";
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
        const {searchNode, searchTxID} = this.props.manaStore;
        return (
            <Container>
                {
                    !nodeStore.status.tangleTime.synced &&
                    <Badge variant="danger">WARNING: Node not in sync, displayed mana values might be outdated!</Badge>
                }
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
                <Row className={"mb-3"}>
                    <Col>
                        <ManaChart
                            node={nodeStore.status.id}
                            data= {manaStore.manaValues.length === 0 ? [[new Date(), 0.0, 0.0]] :manaStore.manaValues}
                        />
                    </Col>
                </Row>
                <Row className="mb-3">
                    <Col>
                        <Card>
                            <Card.Body>
                                <Card.Title>
                                    Filters
                                </Card.Title>
                                <Row>
                                    <Col>
                                        <div>
                                            <Badge pill style={{
                                                backgroundColor: '#41aea9',
                                                color: 'white'
                                            }}>
                                                Events
                                            </Badge>
                                            {' '}
                                            <Badge pill style={{
                                                backgroundColor: '#a6f6f1',
                                                color: 'white'
                                            }}>
                                                Leaderboards
                                            </Badge></div>
                                        <label>
                                            Search Node:
                                        </label>
                                        <input
                                            placeholder="Enter a node ID"
                                            type="text"
                                            value={searchNode}
                                            onChange={(e) => manaStore.updateNodeSearch(e.target.value)}
                                        />
                                    </Col>
                                    <Col>
                                        <div>
                                            <Badge pill style={{
                                                backgroundColor: '#41aea9',
                                                color: 'white'
                                            }}>
                                                Events
                                            </Badge>
                                        </div>
                                        <label>
                                            Search Transaction:
                                        </label>
                                        <input
                                            placeholder="Enter a transaction ID"
                                            type="text"
                                            value={searchTxID}
                                            onChange={(e) => manaStore.updateTxSearch(e.target.value)}
                                        />
                                    </Col>
                                </Row>
                            </Card.Body>
                        </Card>
                    </Col>
                </Row>
                <Row className="mb-3">
                    <Col>
                        <ManaEventList
                            title={"Access Events"}
                            listItems={nodeStore.status.tangleTime.synced?  manaStore.accessEventList: [manaStore.nodeNotSyncedListItem]}
                            since={manaStore.lastRemovedAccessEventTime}
                        />
                    </Col>
                    <Col>
                        <ManaEventList
                            title={"Consensus Events"}
                            listItems={nodeStore.status.tangleTime.synced? manaStore.consensusEventList: [manaStore.nodeNotSyncedListItem]}
                            since={manaStore.lastRemovedConsensusEventTime}
                        />
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaLeaderboard data={manaStore.networkRichestFeedAccess} title={"Access Leaderboard"}/>
                    </Col>
                    <Col>
                        <ManaLeaderboard data={manaStore.networkRichestFeedConsensus} title={"Consensus Leaderboard"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaLeaderboard data={manaStore.activeRichestFeedAccess} title={"Active Access Leaderboard"}/>
                    </Col>
                    <Col>
                        <ManaLeaderboard data={manaStore.activeRichestFeedConsensus} title={"Active Consensus Leaderboard"}/>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ManaHistogram data={manaStore.accessHistogramInput} title={"Access Distribution"}/>
                    </Col>
                    <Col>
                        <ManaHistogram data={manaStore.consensusHistogramInput} title={"Consensus Distribution"}/>
                    </Col>
                </Row>
                <ManaAllowedPledgeID manaStore={manaStore}/>
            </Container>
        );
    }
}
