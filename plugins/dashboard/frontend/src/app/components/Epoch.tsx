import * as React from "react";
import {EpochStore} from "app/stores/EpochStore";
import {inject, observer} from "mobx-react";
import {Col, Container, ListGroup, Row} from "react-bootstrap";
import {Node as ManaNode} from "app/stores/ManaStore"

interface Props {
    epochStore?: EpochStore;
}

@inject("epochStore")
@observer
export class Epoch extends React.Component<Props, any> {
    componentDidMount() {
        this.props.epochStore.getOracleEpoch();
    }
    componentWillUnmount() {
        this.props.epochStore.reset();
    }
    render() {
        let {oracleEpoch} = this.props.epochStore;
        return (
            <Container>
                <h3>Epochs</h3>
                {
                    oracleEpoch &&
                    <Row>
                        <Col>
                            Current Oracle Epoch
                            <ListGroup>
                                <ListGroup.Item>EpochID: {oracleEpoch.epochID}</ListGroup.Item>
                                <ListGroup.Item>Epoch Start Time: {(new Date(oracleEpoch.epochStartTime * 1000).toLocaleString())}</ListGroup.Item>
                                <ListGroup.Item>Epoch End Time: {(new Date(oracleEpoch.epochEndTime * 1000).toLocaleString())}</ListGroup.Item>
                                <ListGroup.Item>Total Weights: {new Intl.NumberFormat().format(oracleEpoch.totalWeight)}</ListGroup.Item>
                                <ListGroup.Item>Total Nodes: {oracleEpoch.weights.length}</ListGroup.Item>
                                <ListGroup.Item>
                                    Weights:
                                    <div style={{overflow: "auto", maxHeight: "500px"}}>
                                        {oracleEpoch.weights.map((weight: ManaNode, index) => {
                                            return <ListGroup style={{marginBottom: "10px", marginTop: "10px"}}>
                                                <ListGroup.Item>Rank: {index +1 }</ListGroup.Item>
                                                <ListGroup.Item>Short NodeID: <strong>{weight.shortNodeID} </strong> Full NodeID: {weight.nodeID}</ListGroup.Item>
                                                <ListGroup.Item>Weight: {new Intl.NumberFormat().format(weight.mana)+" "}</ListGroup.Item>
                                                <ListGroup.Item>Percentage: {(weight.mana/oracleEpoch.totalWeight*100).toPrecision(10)} %</ListGroup.Item>
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