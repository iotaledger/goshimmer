import {inject, observer} from "mobx-react";
import * as React from "react";
import {Badge, Card, Col, ListGroup, ListGroupItem, Row} from "react-bootstrap";
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
                        onClick={() => navigator.clipboard.writeText(value.fullID)}>
                        <div> <b>{value.shortID}</b> (<a>{value.fullID})</a></div>
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
                        onClick={() => navigator.clipboard.writeText(value.fullID)}>
                        <div> <b>{value.shortID}</b> (<a>{value.fullID})</a></div>
                    </ListGroupItem>
                )
            })
        }
        return (
            <Row className={"mb-3"}>
            <Col>
                <Card>
                    <Card.Body>
                        <Card.Title>
                            Access Pledge Filter
                            {' '}
                            {allowedPledgeIDs.accessFilter.enabled ?
                                <Badge variant="success">Enabled</Badge>:
                                <Badge variant="danger">Disabled</Badge>
                            }
                        </Card.Title>
                        <p>Accepted NodeIDs:</p>
                        <ListGroup style={{
                            fontSize: '0.75rem',
                            maxHeight: '150px',
                            overflowY: 'auto'
                        }}>
                            {allowedAccessPledgeIDList}
                        </ListGroup>
                    </Card.Body>
                </Card>
            </Col>
            <Col>
                <Card>
                    <Card.Body>
                        <Card.Title>
                            Consensus Pledge Filter
                            {' '}
                            {allowedPledgeIDs.consensusFilter.enabled ?
                                <Badge variant="success">Enabled</Badge>:
                                <Badge variant="danger">Disabled</Badge>
                            }
                        </Card.Title>
                        <p>Accepted NodeIDs:</p>
                        <ListGroup style={{
                            fontSize: '0.75rem',
                            maxHeight: '150px',
                            overflowY: 'auto'
                        }}>
                            {allowedConsensusPledgeIDList}
                        </ListGroup>
                    </Card.Body>
                </Card>
            </Col>
            </Row>
        );
    }
}