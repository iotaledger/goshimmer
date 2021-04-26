import * as React from "react";
import {EpochData, EpochStore} from "app/stores/EpochStore";
import {inject, observer} from "mobx-react";
import {Col, Container, ListGroup, Row} from "react-bootstrap";
import {Node as ManaNode} from "app/stores/ManaStore"
import * as dateformat from 'dateformat';
import Spinner from "react-bootstrap/Spinner";

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
        let {currentOracleEpoch, previousOracleEpoch, query_loading} = this.props.epochStore;

        return (
            <Container>
                <Row className={"mb-3"}>
                    <Col>
                        {query_loading && <Spinner animation="border"/>}
                    </Col>
                </Row>
                <h3 style={{marginBottom: "10px", marginTop: "20px"}}>Current Oracle Epoch</h3>
                {
                    currentOracleEpoch && <OracleEpoch epoch={currentOracleEpoch}/>
                }
                <h3 style={{marginBottom: "10px", marginTop: "20px"}}>Previous Oracle Epoch</h3>
                {
                    previousOracleEpoch && <OracleEpoch epoch={previousOracleEpoch}/>
                }
            </Container>
        )
    }
}

interface oProps {
    epoch: EpochData;
}

// OracleEpoch displays information of an oracle epoch.
export class OracleEpoch extends React.Component<oProps, any> {
    render() {
        let epoch = this.props.epoch;
        return (
            <div>
                <Row>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>EpochID: {epoch.epochID}</ListGroup.Item>
                            <ListGroup.Item>Epoch Start Time: {dateformat(new Date(epoch.epochStartTime * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                            <ListGroup.Item>Epoch End Time: {dateformat(new Date(epoch.epochEndTime * 1000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                            <ListGroup.Item>Total Active Weight: {new Intl.NumberFormat().format(epoch.totalWeight)}</ListGroup.Item>
                            <ListGroup.Item>Total Active Nodes: {epoch.weights.length}</ListGroup.Item>
                            <ListGroup.Item>
                                Weights:
                                <div style={{overflow: "auto", maxHeight: "500px"}}>
                                    {epoch.weights.map((weight: ManaNode, index) => {
                                        return <ListGroup style={{marginBottom: "10px", marginTop: "10px"}}>
                                            <ListGroup.Item>Rank: {index +1 }</ListGroup.Item>
                                            <ListGroup.Item>Short NodeID: <strong>{weight.shortNodeID} </strong> Full NodeID: {weight.nodeID}</ListGroup.Item>
                                            <ListGroup.Item>Weight: {new Intl.NumberFormat().format(weight.mana)+" "}</ListGroup.Item>
                                            <ListGroup.Item>Percentage: {(weight.mana/epoch.totalWeight*100).toPrecision(10)} %</ListGroup.Item>
                                        </ListGroup>
                                    })
                                    }
                                </div>
                            </ListGroup.Item>
                        </ListGroup>
                    </Col>
                </Row>
            </div>
        )
    }
}