import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from 'react-bootstrap/ListGroup';
import { inject, observer } from 'mobx-react';
import UTXOStore from 'stores/UTXOStore';
import * as dateformat from 'dateformat';
import LinkToDashboard from 'components/LinkToDashboard';

interface Props {
    utxoStore?: UTXOStore;
}

@inject('utxoStore')
@observer
export class TransactionInfo extends React.Component<Props, any> {
    render() {
        const { selectedTx } = this.props.utxoStore;

        return (
            selectedTx && (
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <LinkToDashboard
                                    route={`explorer/transaction/${selectedTx.ID}`}
                                    title={selectedTx.ID}
                                />
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>
                                    Msg ID:
                                    <LinkToDashboard
                                        route={`explorer/messasge/${selectedTx.msgID}`}
                                        title={selectedTx.msgID}
                                    />
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Inputs:
                                    <ListGroup>
                                        {selectedTx.inputs.map((p, i) => (
                                            <ListGroup.Item key={i}>
                                                <LinkToDashboard
                                                    route={`explorer/output/${p.referencedOutputID.base58}`}
                                                    title={
                                                        p.referencedOutputID
                                                            .base58
                                                    }
                                                />
                                            </ListGroup.Item>
                                        ))}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Outputs:
                                    <ListGroup>
                                        {selectedTx.outputs.map((p, i) => (
                                            <ListGroup.Item key={i}>
                                                <LinkToDashboard
                                                    route={`explorer/output/${p}`}
                                                    title={p}
                                                />
                                            </ListGroup.Item>
                                        ))}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    GoF: {selectedTx.gof}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Confirmed:{' '}
                                    {selectedTx.isConfirmed.toString()}
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    GoF updated Time: {' '}
                                    {dateformat(
                                        new Date(
                                            selectedTx.gofTime / 1000000
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
