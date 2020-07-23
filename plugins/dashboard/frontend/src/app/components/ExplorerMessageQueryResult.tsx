import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore, {GenesisMessageID} from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import Badge from "react-bootstrap/Badge";
import * as dateformat from 'dateformat';
import {Link} from 'react-router-dom';
import {BasicPayload} from 'app/components/BasicPayload'
import {DrngPayload} from 'app/components/DrngPayload'
import {ValuePayload} from 'app/components/ValuePayload'
import {SyncBeaconPayload} from 'app/components/SyncBeaconPayload'
import {PayloadType} from 'app/misc/Payload'

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
export class ExplorerMessageQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchMessage(this.props.match.params.id);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.id !== this.props.match.params.id) {
            this.props.explorerStore.searchMessage(this.props.match.params.id);
        }
        return null;
    }

    getPayloadType() {
        switch (this.props.explorerStore.msg.payload_type) {
            case PayloadType.Data:
                return "Data"
            case PayloadType.Value:
                return "Value"
            case PayloadType.Drng:
                return "Drng"
            case PayloadType.Faucet:
                return "Faucet"
            case PayloadType.SyncBeacon:
                return "SyncBeacon"
            default:
                return "Unknown"
        }
    }

    renderPayload() {
        switch (this.props.explorerStore.msg.payload_type) {
            case PayloadType.Drng:
                return <DrngPayload/>
            case PayloadType.Value:
                return <ValuePayload/>
            case PayloadType.Data:
            case PayloadType.SyncBeacon:
                return <SyncBeaconPayload/>
            case PayloadType.Faucet:
            default:
                return <BasicPayload/>
        }
    }

    render() {
        let {id} = this.props.match.params;
        let {msg, query_loading, query_err} = this.props.explorerStore;

        if (id === GenesisMessageID) {
            return (
                <Container>
                    <h3>Genesis Message</h3>
                    <p>In the beginning there was the genesis.</p>
                </Container>
            );
        }

        if (query_err) {
            return (
                <Container>
                    <h3>Message not available - 404</h3>
                    <p>
                        Message with ID {id} not found.
                    </p>
                </Container>
            );
        }

        return (
            <Container>
                <h3>Message</h3>
                <p>
                    {id} {' '}
                    {
                        msg &&
                        <React.Fragment>
                            <br/>
                            <span>
                                <Badge variant="light" style={{marginRight: 10}}>
                                   Issuance Time: {dateformat(new Date(msg.issuance_timestamp * 1000), "dd.mm.yyyy HH:MM:ss")}
                                </Badge>
                                <Badge variant="light">
                                   Solidification Time: {dateformat(new Date(msg.solidification_timestamp * 1000), "dd.mm.yyyy HH:MM:ss")}
                                </Badge>
                            </span>
                        </React.Fragment>
                    }
                </p>
                {
                    msg &&
                    <React.Fragment>
                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Payload Type: {this.getPayloadType()}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Sequence Number: {msg.sequence_number}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Solid: {msg.solid ? 'Yes' : 'No'}
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>

                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item>
                                        Issuer Public Key: {msg.issuer_public_key}
                                    </ListGroup.Item>
                                    <ListGroup.Item>
                                        Message Signature: {msg.signature}
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>

                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item className="text-break">
                                        Parent 1 Message ID: {' '}
                                        <Link to={`/explorer/message/${msg.trunk_message_id}`}>
                                            {msg.trunk_message_id}
                                        </Link>
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item className="text-break">
                                        Parent 2 Message ID: {' '}
                                        <Link to={`/explorer/message/${msg.branch_message_id}`}>
                                            {msg.branch_message_id}
                                        </Link>
                                    </ListGroup.Item>
                                </ListGroup>
                            </Col>
                        </Row>

                        <Row className={"mb-3"}>
                            <Col>
                                <h4>Payload</h4>
                            </Col>
                        </Row>

                        <Row className={"mb-3"}>
                            <Col>
                                <ListGroup>
                                    <ListGroup.Item className="text-break">
                                        {this.renderPayload()}
                                    </ListGroup.Item>
                                </ListGroup>
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
