import * as React from "react";
import {EpochStore} from "app/stores/EpochStore";
import {inject, observer} from "mobx-react";
import {Col, Container, ListGroup, Row} from "react-bootstrap";
import {Node as ManaNode} from "app/stores/ManaStore"
import * as dateformat from 'dateformat';

interface Props {
    epochStore?: EpochStore;
}

@inject("epochStore")
@observer
export class Epoch extends React.Component<Props, any> {
    componentDidMount() {
        this.props.epochStore.getOracleEpochs();
    }
    componentWillUnmount() {
        this.props.epochStore.reset();
    }
    render() {
        let {currentOracleEpoch, previousOracleEpoch} = this.props.epochStore;

        return (
            <Container>
                <h3>Current Oracle Epoch</h3>
                {
                    currentOracleEpoch &&
                    <Row>
                        <Col>
                            <ListGroup>
                                <ListGroup.Item>EpochID: {currentOracleEpoch.epochID}</ListGroup.Item>
                                <ListGroup.Item>Epoch Start Time: {dateformat(new Date(currentOracleEpoch.epochStartTime * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                                <ListGroup.Item>Epoch End Time: {dateformat(new Date(currentOracleEpoch.epochEndTime * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                                <ListGroup.Item>Total Active Weight: {new Intl.NumberFormat().format(currentOracleEpoch.totalWeight)}</ListGroup.Item>
                                <ListGroup.Item>Total Active Nodes: {currentOracleEpoch.weights.length}</ListGroup.Item>
                                <ListGroup.Item>
                                    Weights:
                                    <div style={{overflow: "auto", maxHeight: "500px"}}>
                                        {currentOracleEpoch.weights.map((weight: ManaNode, index) => {
                                            return <ListGroup style={{marginBottom: "10px", marginTop: "10px"}}>
                                                <ListGroup.Item>Rank: {index +1 }</ListGroup.Item>
                                                <ListGroup.Item>Short NodeID: <strong>{weight.shortNodeID} </strong> Full NodeID: {weight.nodeID}</ListGroup.Item>
                                                <ListGroup.Item>Weight: {new Intl.NumberFormat().format(weight.mana)+" "}</ListGroup.Item>
                                                <ListGroup.Item>Percentage: {(weight.mana/currentOracleEpoch.totalWeight*100).toPrecision(10)} %</ListGroup.Item>
                                            </ListGroup>
                                        })
                                        }
                                    </div>
                                </ListGroup.Item>
                            </ListGroup>
                        </Col>
                    </Row>
                }
                <hr/>
                <hr/>
                <h3>Previous Oracle Epoch</h3>
                {
                    previousOracleEpoch &&
                    <Row>
                        <Col>
                            <ListGroup>
                                <ListGroup.Item>EpochID: {previousOracleEpoch.epochID}</ListGroup.Item>
                                <ListGroup.Item>Epoch Start Time: {dateformat(new Date(previousOracleEpoch.epochStartTime * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                                <ListGroup.Item>Epoch End Time: {dateformat(new Date(previousOracleEpoch.epochEndTime * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                                <ListGroup.Item>Total Active Weight: {new Intl.NumberFormat().format(previousOracleEpoch.totalWeight)}</ListGroup.Item>
                                <ListGroup.Item>Total Active Nodes: {previousOracleEpoch.weights.length}</ListGroup.Item>
                                <ListGroup.Item>
                                    Weights:
                                    <div style={{overflow: "auto", maxHeight: "500px"}}>
                                        {previousOracleEpoch.weights.map((weight: ManaNode, index) => {
                                            return <ListGroup style={{marginBottom: "10px", marginTop: "10px"}}>
                                                <ListGroup.Item>Rank: {index +1 }</ListGroup.Item>
                                                <ListGroup.Item>Short NodeID: <strong>{weight.shortNodeID} </strong> Full NodeID: {weight.nodeID}</ListGroup.Item>
                                                <ListGroup.Item>Weight: {new Intl.NumberFormat().format(weight.mana)+" "}</ListGroup.Item>
                                                <ListGroup.Item>Percentage: {(weight.mana/previousOracleEpoch.totalWeight*100).toPrecision(10)} %</ListGroup.Item>
                                            </ListGroup>
                                        })
                                        }
                                    </div>
                                </ListGroup.Item>
                            </ListGroup>
                        </Col>
                    </Row>
                }
            </Container>
        )
    }
}