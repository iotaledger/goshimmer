import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from "react-bootstrap/ListGroup";
import {inject, observer} from "mobx-react";
import BranchStore from "stores/BranchStore";

interface Props {
    branchStore?: BranchStore;
}

@inject("branchStore")
@observer
export class BranchInfo extends React.Component<Props, any> {
    render () {
        let { selectedBranch, explorerAddress } = this.props.branchStore;

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
                                        {selectedBranch.parents.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/branch/${p}`} target="_blank" rel="noopener noreferrer">{p}</a></ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>Confirmed: {selectedBranch.isConfirmed.toString()}</ListGroup.Item>
                                <ListGroup.Item>GoF: {selectedBranch.gof}</ListGroup.Item>
                                <ListGroup.Item>AW: {selectedBranch.aw}</ListGroup.Item>
                                { selectedBranch.type === "ConflictBranchType" && selectedBranch.conflicts &&
                                    <ListGroup.Item>
                                        Conflicts:
                                            { selectedBranch.conflicts.conflicts.map((p,i) => {
                                                    return (
                                                        <ListGroup key={i}>
                                                            <ListGroup.Item>OutputID: <a href={`${explorerAddress}/explorer/output/${p}`} target="_blank" rel="noopener noreferrer">{p.outputID.base58}</a></ListGroup.Item>
                                                            <ListGroup.Item>Branches:
                                                                <ListGroup>
                                                                    {p.branchIDs.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/branch/${p}`} target="_blank" rel="noopener noreferrer">{p}</a></ListGroup.Item>)}
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