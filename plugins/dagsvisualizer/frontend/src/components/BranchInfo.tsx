import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from "react-bootstrap/ListGroup";
import {inject, observer} from "mobx-react";
import BranchStore from "stores/BranchStore";
import * as dateformat from 'dateformat';

interface Props {
    branchStore?: BranchStore;
}

@inject("branchStore")
@observer
export class BranchInfo extends React.Component<Props, any> {
    render () {
        let { selectedBranch } = this.props.branchStore;

        return (
            selectedBranch  &&
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>{selectedBranch.ID}</Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>Type: {selectedBranch.type}</ListGroup.Item>
                                <ListGroup.Item>
                                    Parent:
                                    <ListGroup>
                                        {selectedBranch.parents.map((p,i) => <ListGroup.Item key={i}>{p}</ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>Approval Weight: {selectedBranch.approvalWeight}</ListGroup.Item>
                                <ListGroup.Item>Confirmed Time: {dateformat(new Date(selectedBranch.confirmedTime/1000000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                                { selectedBranch.type === "ConflictBranchType" && selectedBranch.conflicts &&
                                    <ListGroup.Item>
                                        Conflicts:
                                            { selectedBranch.conflicts.conflicts.map((p,i) => {
                                                    return (
                                                        <ListGroup>
                                                            <ListGroup.Item>OutputID: {p.outputID.base58}</ListGroup.Item>
                                                            <ListGroup.Item>Branches:
                                                                <ListGroup>
                                                                    {p.branchIDs.map((p,i) => <ListGroup.Item key={i}>{p}</ListGroup.Item>)}
                                                                </ListGroup>
                                                            </ListGroup.Item>
                                                        </ListGroup>
                                                )})
                                            }
                                    </ListGroup.Item>
                                }
                            </ListGroup>
                        </Card.Body>
                    </Card>
                </div> 
        );
    }
}