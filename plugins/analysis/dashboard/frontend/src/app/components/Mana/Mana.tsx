import * as React from 'react'
import {inject, observer} from "mobx-react";
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import {Badge, Card, Col} from "react-bootstrap";
import ManaStore from "../../stores/ManaStore";
import ManaGauge from "./ManaGauge";
import ManaEventList from "./ManaEventList";
import ManaLeaderboard from "./ManaLeaderboard";
import ManaHistogram from "./ManaHistogram";

interface Props {
    manaStore: ManaStore;
}

@inject("manaStore")
@observer
export default class Mana extends React.Component<Props, any> {
    render() {
        let manaStore = this.props.manaStore
        const {searchNode, searchTxID} = manaStore
        return (
            <Container>
                <Row className={"mb-3 mt-3"}>
                    <Col>
                        <ManaGauge
                            data={manaStore.manaValues.length === 0 ? [0.0, 0.0] :[manaStore.manaValues[manaStore.manaValues.length -1][1], manaStore.prevManaValues[0]]}
                            title={"Total Access Mana in Network"}
                        />
                    </Col>
                    <Col>
                        <ManaGauge
                            data={manaStore.manaValues.length === 0 ? [0.0, 0.0] :[manaStore.manaValues[manaStore.manaValues.length -1][2], manaStore.prevManaValues[1]]}
                            title={"Total Consensus Mana in Network"}
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
            </Container>
        );
    }
}
