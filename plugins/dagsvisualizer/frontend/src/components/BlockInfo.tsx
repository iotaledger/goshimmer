import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import { inject, observer } from 'mobx-react';
import TangleStore from 'stores/TangleStore';
import { resolveBase58ConflictID } from 'utils/ConflictIDResolver';
import * as dateformat from 'dateformat';
import LinkToDashboard from 'components/LinkToDashboard';

interface Props {
    tangleStore?: TangleStore;
}

@inject('tangleStore')
@observer
export class BlockInfo extends React.Component<Props, any> {
    render() {
        const { selectedBlk } = this.props.tangleStore;

        return (
            selectedBlk && (
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <LinkToDashboard
                                    route={`explorer/block/${selectedBlk.ID}`}
                                    title={selectedBlk.ID}
                                />
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Strong Parents:
                                    <ListGroup>
                                        {selectedBlk.strongParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/block/${p}`}
                                                        title={p}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Weak Parents:
                                    <ListGroup>
                                        {selectedBlk.weakParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/block/${p}`}
                                                        title={p}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    ShallowLike Parents:
                                    <ListGroup>
                                        {selectedBlk.shallowLikeParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/block/${p}`}
                                                        title={p}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                {selectedBlk.isTx && (
                                    <ListGroup.Item>
                                        Transaction:{' '}
                                        <LinkToDashboard
                                            route={`explorer/transaction/${selectedBlk.txID}`}
                                            title={selectedBlk.txID}
                                        />
                                    </ListGroup.Item>
                                )}
                                <ListGroup.Item>
                                    ConflictIDs:{' '}
                                    <ListGroup>
                                        {selectedBlk.conflictIDs && selectedBlk.conflictIDs.map(
                                            (b, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/conflict/${b}`}
                                                        title={resolveBase58ConflictID(
                                                            b
                                                        )}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    isMarker: {selectedBlk.isMarker.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    ConfirmationState: {selectedBlk.confirmationState}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Confirmed:{' '}
                                    {selectedBlk.isConfirmed.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Confirmed Time:{' '}
                                    {dateformat(
                                        new Date(
                                            selectedBlk.confirmationStateTime / 1000000
                                        ),
                                        'dd.mm.yyyy HH:MM:ss'
                                    )}
                                </ListGroup.Item>
                            </ListGroup>
                        </Card.Body>
                    </Card>
                </div>
            )
        );
    }
}
