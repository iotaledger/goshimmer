import {inject, observer} from "mobx-react";
import * as React from "react";
import {Card, Col, ListGroup, ListGroupItem, Row} from "react-bootstrap";
import ManaStore from "app/stores/ManaStore";

interface Props {
    manaStore: ManaStore;
}

@inject("manaStore")
@observer
export default class ManaAllowedPledgeID extends React.Component<Props, any> {
    render() {
        if (this.props.manaStore.allowedPledgeIDs === undefined) {
            return [];
        }
        let allowedPledgeIDs = this.props.manaStore.allowedPledgeIDs;
        let allowedAccessPledgeIDList = [];
        if (!allowedPledgeIDs.accessFilter.enabled) {
            allowedAccessPledgeIDList.push(
                <ListGroupItem key={"empty-allowed-access"}>
                    Node accepts any access mana pledgeID.
                </ListGroupItem>
            )
        } else {
            allowedPledgeIDs.accessFilter.allowedNodeIDs.forEach( (value) => {
                allowedAccessPledgeIDList.push(
                    <ListGroupItem
                        key={value.shortID}
                        as={'button'}
                        onClick={() => navigator.clipboard.writeText(value.fullID)}>
                        <div>NodeID: {value.shortID}</div>
                        <div>Base58 Encoded Full NodeID: {value.fullID}</div>
                    </ListGroupItem>
                )
            })
        }
        let allowedConsensusPledgeIDList = [];
        if (!allowedPledgeIDs.consensusFilter.enabled) {
            allowedConsensusPledgeIDList.push(
                <ListGroupItem key={"empty-allowed-consensus"}>
                    Node accepts any consensus mana pledgeID.
                </ListGroupItem>
            )
        } else {
            allowedPledgeIDs.consensusFilter.allowedNodeIDs.forEach( (value) => {
                allowedConsensusPledgeIDList.push(
                    // TODO: align left
                    <ListGroupItem
                        key={value.shortID}
                        as={'button'}
                        onClick={() => navigator.clipboard.writeText(value.fullID)}>
                        <h5>NodeID: </h5>{value.shortID}
                        <h5>Base58 Encoded Full NodeID: </h5>{value.fullID}
                    </ListGroupItem>
                )
            })
        }
        return (
            <Row>
            <Col>
                <Card>
                    <Card.Body>
                        <Card.Title>
                            Access Mana Pledge NodeID Filter
                        </Card.Title>
                        <p>Enabled: {allowedPledgeIDs.accessFilter.enabled ? 'true': 'false'}</p>
                        <p>Accepted Access Pledge NodeIDs:</p>
                        <ListGroup>
                            {allowedAccessPledgeIDList}
                        </ListGroup>
                    </Card.Body>
                </Card>
            </Col>
            <Col>
                <Card>
                    <Card.Body>
                        <Card.Title>
                            Consensus Mana Pledge NodeID Filter
                        </Card.Title>
                        <p>Enabled: {allowedPledgeIDs.consensusFilter.enabled ? 'true': 'false'}</p>
                        <p>Accepted Consensus Pledge NodeIDs:</p>
                        <ListGroup>
                            {allowedConsensusPledgeIDList}
                        </ListGroup>
                    </Card.Body>
                </Card>
            </Col>
            </Row>
        );
    }
}