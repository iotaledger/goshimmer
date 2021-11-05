import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from "react-bootstrap/ListGroup";
import {inject, observer} from "mobx-react";
import TangleStore from "stores/TangleStore";
import {resolveBase58BranchID} from 'utils';
import * as dateformat from 'dateformat';

interface Props {
    tangleStore?: TangleStore;
}

@inject("tangleStore")
@observer
export class MessageInfo extends React.Component<Props, any> {
    render () {
        let { selectedMsg, selected_via_click } = this.props.tangleStore;

        return (
            selectedMsg && selected_via_click &&
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>{selectedMsg.ID}</Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Strong Parents:
                                    <ListGroup>
                                        {selectedMsg.strongParentIDs.map((p,i) => <ListGroup.Item key={i}>{p}</ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Weak Parents:
                                    <ListGroup>
                                        {selectedMsg.weakParentIDs.map((p,i) => <ListGroup.Item key={i}>{p}</ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Liked Parents:
                                    <ListGroup>
                                        {selectedMsg.likedParentIDs.map((p,i) => <ListGroup.Item key={i}>{p}</ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>Branch: {resolveBase58BranchID(selectedMsg.branchID)}</ListGroup.Item>
                                <ListGroup.Item>isMarker: {selectedMsg.isMarker.toString()}</ListGroup.Item>
                                <ListGroup.Item>GoF: {selectedMsg.gof}</ListGroup.Item>
                                <ListGroup.Item>Confirmed Time: {dateformat(new Date(selectedMsg.confirmedTime/1000000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                            </ListGroup>
                        </Card.Body>
                    </Card>
                </div> 
        );
    }
}