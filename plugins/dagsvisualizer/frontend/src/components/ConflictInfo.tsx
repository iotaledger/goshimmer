import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import { inject, observer } from 'mobx-react';
import ConflictStore from 'stores/ConflictStore';
import LinkToDashboard from 'components/LinkToDashboard';
import {resolveBase58ConflictID} from '../utils/ConflictIDResolver';

interface Props {
    conflictStore?: ConflictStore;
}

@inject('conflictStore')
@observer
export class ConflictInfo extends React.Component<Props, any> {
    render() {
        const { selectedConflict } = this.props.conflictStore;

        return (
            selectedConflict && (
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <LinkToDashboard
                                    route={`explorer/conflict/${
                                        resolveBase58ConflictID(selectedConflict.ID)
                                    }`}
                                    title={selectedConflict.ID}
                                />
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Parent:
                                    <ListGroup>
                                        {selectedConflict.parents ? (
                                            selectedConflict.parents.map(
                                                (p, i) => (
                                                    <ListGroup.Item key={i}>
                                                        <LinkToDashboard
                                                            route={`explorer/conflict/${p}`}
                                                            title={resolveBase58ConflictID(p)}
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
                                    {selectedConflict.isConfirmed.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    ConfirmationState: {selectedConflict.confirmationState}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    AW: {selectedConflict.aw}
                                </ListGroup.Item>
                                {selectedConflict.conflicts && (
                                    <ListGroup.Item>
                                            Conflicts:
                                        {selectedConflict.conflicts.conflicts.map(
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
                                                                Conflicts:
                                                            <ListGroup>
                                                                {p.conflictIDs.map(
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
                                                                                route={`explorer/conflict/${p}`}
                                                                                title={
                                                                                    resolveBase58ConflictID(p)
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
