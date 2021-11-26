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
        let { selectedMsg, selected_via_click,explorerAddress } = this.props.tangleStore;

        return (
            selectedMsg && selected_via_click &&
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <a href={`${explorerAddress}/explorer/message/${selectedMsg.ID}`} target="_blank" rel="noopener noreferrer">
                                    {selectedMsg.ID}
                                </a>
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Strong Parents:
                                    <ListGroup>
                                        {selectedMsg.strongParentIDs.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/message/${p}`} target="_blank" rel="noopener noreferrer">{p}</a></ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Weak Parents:
                                    <ListGroup>
                                        {selectedMsg.weakParentIDs.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/message/${p}`} target="_blank" rel="noopener noreferrer">{p}</a></ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Liked Parents:
                                    <ListGroup>
                                        {selectedMsg.likedParentIDs.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/message/${p}`} target="_blank" rel="noopener noreferrer">{p}</a></ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>Branch: <a href={`${explorerAddress}/explorer/branch/${selectedMsg.branchID}`} target="_blank" rel="noopener noreferrer">{resolveBase58BranchID(selectedMsg.branchID)}</a></ListGroup.Item>
                                <ListGroup.Item>isMarker: {selectedMsg.isMarker.toString()}</ListGroup.Item>
                                <ListGroup.Item>GoF: {selectedMsg.gof}</ListGroup.Item>
                                <ListGroup.Item>Confrimed: {selectedMsg.isConfirmed.toString()}</ListGroup.Item>                                
                                <ListGroup.Item>Confirmed Time: {dateformat(new Date(selectedMsg.confirmedTime/1000000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                            </ListGroup>
                        </Card.Body>
                    </Card>
                </div> 
        );
    }
}