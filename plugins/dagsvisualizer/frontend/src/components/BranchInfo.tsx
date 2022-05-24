import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import { inject, observer } from 'mobx-react';
import BranchStore from 'stores/BranchStore';
import LinkToDashboard from 'components/LinkToDashboard';
import {resolveBase58BranchID} from '../utils/BranchIDResolver';

interface Props {
    branchStore?: BranchStore;
}

@inject('branchStore')
@observer
export class BranchInfo extends React.Component<Props, any> {
    render() {
        const { selectedBranch } = this.props.branchStore;

        return (
            selectedBranch && (
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <LinkToDashboard
                                    route={`explorer/branch/${
                                        resolveBase58BranchID(selectedBranch.ID)
                                    }`}
                                    title={selectedBranch.ID}
                                />
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Parent:
                                    <ListGroup>
                                        {selectedBranch.parents ? (
                                            selectedBranch.parents.map(
                                                (p, i) => (
                                                    <ListGroup.Item key={i}>
                                                        <LinkToDashboard
                                                            route={`explorer/branch/${p}`}
                                                            title={resolveBase58BranchID(p)}
                                                        />
                                                    </ListGroup.Item>
                                                )
                                            )
                                        ) : (
                                            <></>
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Confirmed:{' '}
                                    {selectedBranch.isConfirmed.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    GoF: {selectedBranch.gof}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    AW: {selectedBranch.aw}
                                </ListGroup.Item>
                                {selectedBranch.conflicts && (
                                    <ListGroup.Item>
                                            Conflicts:
                                        {selectedBranch.conflicts.conflicts.map(
                                            (p, i) => {
                                                return (
                                                    <ListGroup key={i}>
                                                        <ListGroup.Item>
                                                                OutputID:{' '}
                                                            <LinkToDashboard
                                                                route={`explorer/output/${p}`}
                                                                title={
                                                                    p
                                                                        .outputID
                                                                        .base58
                                                                }
                                                            />
                                                        </ListGroup.Item>
                                                        <ListGroup.Item>
                                                                Branches:
                                                            <ListGroup>
                                                                {p.branchIDs.map(
                                                                    (
                                                                        p,
                                                                        i
                                                                    ) => (
                                                                        <ListGroup.Item
                                                                            key={
                                                                                i
                                                                            }
                                                                        >
                                                                            <LinkToDashboard
                                                                                route={`explorer/branch/${p}`}
                                                                                title={
                                                                                    resolveBase58BranchID(p)
                                                                                }
                                                                            />
                                                                        </ListGroup.Item>
                                                                    )
                                                                )}
                                                            </ListGroup>
                                                        </ListGroup.Item>
                                                    </ListGroup>
                                                );
                                            }
                                        )}
                                    </ListGroup.Item>
                                )}
                            </ListGroup>
                        </Card.Body>
                    </Card>
                </div>
            )
        );
    }
}
