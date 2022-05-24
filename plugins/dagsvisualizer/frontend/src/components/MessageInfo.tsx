import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import { inject, observer } from 'mobx-react';
import TangleStore from 'stores/TangleStore';
import { resolveBase58BranchID } from 'utils/BranchIDResolver';
import * as dateformat from 'dateformat';
import LinkToDashboard from 'components/LinkToDashboard';

interface Props {
    tangleStore?: TangleStore;
}

@inject('tangleStore')
@observer
export class MessageInfo extends React.Component<Props, any> {
    render() {
        const { selectedMsg } = this.props.tangleStore;

        return (
            selectedMsg && (
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <LinkToDashboard
                                    route={`explorer/message/${selectedMsg.ID}`}
                                    title={selectedMsg.ID}
                                />
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Strong Parents:
                                    <ListGroup>
                                        {selectedMsg.strongParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/message/${p}`}
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
                                        {selectedMsg.weakParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/message/${p}`}
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
                                        {selectedMsg.shallowLikeParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/message/${p}`}
                                                        title={p}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    ShallowDislike Parents:
                                    <ListGroup>
                                        {selectedMsg.shallowDislikeParentIDs.map(
                                            (p, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/message/${p}`}
                                                        title={p}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                {selectedMsg.isTx && (
                                    <ListGroup.Item>
                                        Transaction:{' '}
                                        <LinkToDashboard
                                            route={`explorer/transaction/${selectedMsg.txID}`}
                                            title={selectedMsg.txID}
                                        />
                                    </ListGroup.Item>
                                )}
                                <ListGroup.Item>
                                    BranchIDs:{' '}
                                    <ListGroup>
                                        {selectedMsg.branchIDs.map(
                                            (b, i) => (
                                                <ListGroup.Item key={i}>
                                                    <LinkToDashboard
                                                        route={`explorer/branch/${b}`}
                                                        title={resolveBase58BranchID(
                                                            b
                                                        )}
                                                    />
                                                </ListGroup.Item>
                                            )
                                        )}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    isMarker: {selectedMsg.isMarker.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    GoF: {selectedMsg.gof}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Confirmed:{' '}
                                    {selectedMsg.isConfirmed.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Confirmed Time:{' '}
                                    {dateformat(
                                        new Date(
                                            selectedMsg.confirmedTime / 1000000
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
