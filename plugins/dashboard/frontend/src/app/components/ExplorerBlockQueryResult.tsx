import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore, {GenesisBlockID} from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import Badge from "react-bootstrap/Badge";
import * as dateformat from 'dateformat';
import {Link} from 'react-router-dom';
import {BasicPayload} from 'app/components/BasicPayload'
import {TransactionPayload} from 'app/components/TransactionPayload'
import {ChatPayload} from 'app/components/ChatPayload'
import {getPayloadType, PayloadType} from 'app/misc/Payload'
import {StatementPayload} from "app/components/StatemenetPayload";
import {resolveBase58ConflictID} from "app/utils/conflict";
import {resolveConfirmationState} from "app/utils/confirmation_state";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            id: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerBlockQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchBlock(this.props.match.params.id);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.id !== this.props.match.params.id) {
            this.props.explorerStore.searchBlock(this.props.match.params.id);
        }
        return null;
    }

    getPayloadType() {
        return getPayloadType(this.props.explorerStore.blk.payload_type)
    }

    renderPayload() {
        switch (this.props.explorerStore.blk.payload_type) {
            case PayloadType.Transaction:
                if (!this.props.explorerStore.blk.objectivelyInvalid) {
                    return <TransactionPayload/>
                }
                return <BasicPayload/>
            case PayloadType.Statement:
                return <StatementPayload/>
            case PayloadType.Data:
                return <BasicPayload/>
            case PayloadType.Faucet:
            case PayloadType.Chat:
                return <ChatPayload/>
            default:
                return <BasicPayload/>
        }
    }

    render() {
        let {id} = this.props.match.params;
        let {blk, query_loading, query_err} = this.props.explorerStore;

        if (id === GenesisBlockID) {
            return (
                <Container>
                    <h3>Genesis Block</h3>
                    <p>In the beginning there was the genesis.</p>
                </Container>
            );
        }

        if (query_err) {
            return (
                <Container>
                    <h3>Block not available - 404</h3>
                    <p>
                        Block with ID {id} not found.
                    </p>
                </Container>
            );
        }
        return (
            <Container>
                <h3>Block</h3>
                <p>
                    {id} {' '}
                    {
                        blk &&
                        <React.Fragment>
                            <br/>
                            <span>
                                <Badge variant="light" style={{marginRight: 10}}>
                                   Issuance Time: {dateformat(new Date(blk.issuance_timestamp * 1000), "dd.mm.yyyy HH:MM:ss")}
                                </Badge>
                                <Badge variant="light">
                                   Solidification Time: {dateformat(new Date(blk.solidification_timestamp * 1000), "dd.mm.yyyy HH:MM:ss")}
                                </Badge>
                            </span>
                        </React.Fragment>
                    }
                </p>
                {
                    blk &&
                    <React.Fragment>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Payload Type: {this.getPayloadType()}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Sequence Number: {blk.sequence_number}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        ConflictIDs:
                                        <ListGroup>
                                            {
                                                blk.conflictIDs.map((value, index) => {
                                                    return (
                                                        <ListGroup.Item key={"ConflictID" + index + 1} className="text-break">
                                                            <Link to={`/explorer/conflict/${value}`}>
                                                                {resolveBase58ConflictID(value)}
                                                            </Link>
                                                        </ListGroup.Item>
                                                    )
                                                })
                                            }
                                        </ListGroup>
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        AddedConflictIDs:
                                        <ListGroup>
                                            {
                                                blk.addedConflictIDs.map((value, index) => {
                                                    return (
                                                        <ListGroup.Item key={"AddedConflictID" + index + 1} className="text-break">
                                                            <Link to={`/explorer/conflict/${value}`}>
                                                                {resolveBase58ConflictID(value)}
                                                            </Link>
                                                        </ListGroup.Item>
                                                    )
                                                })
                                            }
                                        </ListGroup>
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        SubtractedConflictIDs:
                                        <ListGroup>
                                            {
                                                blk.subtractedConflictIDs.map((value, index) => {
                                                    return (
                                                        <ListGroup.Item key={"SubtractedConflictID" + index + 1} className="text-break">
                                                            <Link to={`/explorer/conflict/${value}`}>
                                                                {resolveBase58ConflictID(value)}
                                                            </Link>
                                                        </ListGroup.Item>
                                                    )
                                                })
                                            }
                                        </ListGroup>
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Solid: {blk.solid ? 'Yes' : 'No'}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Scheduled: {blk.scheduled ? 'Yes' : 'No'}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Booked: {blk.booked ? 'Yes' : 'No'}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Objectively Invalid: {blk.objectivelyInvalid ? 'Yes' : 'No'}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Subjectively Invalid: {blk.subjectivelyInvalid ? 'Yes' : 'No'}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Confirmation State: {resolveConfirmationState(blk.confirmationState)}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Confirmation State Time: {dateformat(new Date(blk.confirmationStateTime * 1000), "dd.mm.yyyy HH:MM:ss")}
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>

                        {
                            <Row className={"mb-3"}>
                                <Col>
                                    <h5>Epoch Commitment</h5>
                                    <ListGroup>
                                        <ListGroup.Item>
                                            EC: {blk.ec}
                                        </ListGroup.Item>
                                        <ListGroup.Item>
                                            <ListGroup>
                                                <ListGroup.Item>
                                                    EI: {blk.ei}
                                                </ListGroup.Item>
                                                <ListGroup.Item>
                                                    ECR: {blk.ecr}
                                                </ListGroup.Item>
                                                <ListGroup.Item>
                                                    PrevEC: {blk.prevEC}
                                                </ListGroup.Item>
                                            </ListGroup>
                                        </ListGroup.Item>
                                        <ListGroup.Item>
                                            LatestConfirmedEpoch: {blk.latestConfirmedEpoch}
                                        </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                            </Row>
                        }

                        {
                            !!blk.rank &&
                            <Row className={"mb-3"}>
                                <Col>
                                    <h5>Markers</h5>
                                    <ListGroup>
                                        <ListGroup.Item>
                                            Rank: {blk.rank}
                                        </ListGroup.Item>
                                        <ListGroup.Item>
                                            SequenceID: {blk.sequenceID}
                                        </ListGroup.Item>
                                        <ListGroup.Item>
                                            PastMarkerGap: {blk.pastMarkerGap}
                                        </ListGroup.Item>
                                        <ListGroup.Item>
                                            IsPastMarker: {blk.isPastMarker ? 'Yes' : 'No'}
                                        </ListGroup.Item>
                                        <ListGroup.Item>
                                            Past markers: {blk.pastMarkers}
                                        </ListGroup.Item>
                                    </ListGroup>
                                </Col>
                            </Row>
                        }


                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Issuer Public Key: {blk.issuer_public_key}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Issuer NodeID: {blk.issuer_short_id}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Block Signature: {blk.signature}
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>
                        {
                            Object.keys(blk.parentsByType).map(parentsTypeName =>
                                <Row className={"mb-3"}>
                                    <Col>
                                        <ListGroup>
                                            {
                                                blk.parentsByType[parentsTypeName].map((value, index) =>
                                                    <ListGroup.Item className="text-break">
                                                        {parentsTypeName} {index + 1}: {' '}
                                                        <Link
                                                            to={`/explorer/block/${blk.parentsByType[parentsTypeName][index]}`}>
                                                            {blk.parentsByType[parentsTypeName][index]}
                                                        </Link>
                                                    </ListGroup.Item>
                                                )
                                            }
                                        </ListGroup>
                                    </Col>
                                </Row>
                            )
                        }

                        <Row>
                            <Col>
                                <ListGroup>
                                    {
                                        blk.strongChildren.map((value, index) => {
                                            return (
                                                <ListGroup.Item key={"Strong Child" + index + 1}
                                                                className="text-break">
                                                    Strong Child {index + 1}: {' '}
                                                    <Link to={`/explorer/block/${blk.strongChildren[index]}`}>
                                                        {blk.strongChildren[index]}
                                                    </Link>
                                                </ListGroup.Item>
                                            )
                                        })
                                    }
                                </ListGroup>
                            </Col>
                        </Row>

                        <Row>
                            <Col>
                                <ListGroup>
                                    {
                                        blk.weakChildren.map((value, index) => {
                                            return (
                                                <ListGroup.Item key={"Weak Child" + index + 1}
                                                                className="text-break">
                                                    Weak Child {index + 1}: {' '}
                                                    <Link to={`/explorer/block/${blk.weakChildren[index]}`}>
                                                        {blk.weakChildren[index]}
                                                    </Link>
                                                </ListGroup.Item>
                                            )
                                        })
                                    }
                                </ListGroup>
                            </Col>
                        </Row>

                        <Row>
                            <Col>
                                <ListGroup>
                                    {
                                        blk.shallowLikeChildren.map((value, index) => {
                                            return (
                                                <ListGroup.Item key={"ShallowLike Child" + index + 1}
                                                                className="text-break">
                                                    ShallowLike Child {index + 1}: {' '}
                                                    <Link to={`/explorer/block/${blk.shallowLikeChildren[index]}`}>
                                                        {blk.shallowLikeChildren[index]}
                                                    </Link>
                                                </ListGroup.Item>
                                            )
                                        })
                                    }
                                </ListGroup>
                            </Col>
                        </Row>

                        <Row className={"mb-3"} style={{marginTop: "20px", marginBottom: "20px"}}>
                            <Col>
                                <h3>Payload</h3>
                            </Col>
                        </Row>
                        <Row className={"mb-3"}>
                            <Col>
                                {this.renderPayload()}
                            </Col>
                        </Row>
                    </React.Fragment>
                }
                <Row className={"mb-3"}>
                    <Col>
                        {query_loading && <Spinner animation="border"/>}
                    </Col>
                </Row>
            </Container>
        );
    }
}
