import * as React from 'react';
import Card from 'react-bootstrap/Card';
import ListGroup from "react-bootstrap/ListGroup";
import {inject, observer} from "mobx-react";
import UTXOStore from "stores/UTXOStore";
import * as dateformat from 'dateformat';

interface Props {
    utxoStore?: UTXOStore;
}

@inject("utxoStore")
@observer
export class TransactionInfo extends React.Component<Props, any> {
    render () {
        let { selectedTx, explorerAddress } = this.props.utxoStore;

        return (
            selectedTx  &&
                <div className="selectedInfo">
                    <Card style={{ width: '100%' }}>
                        <Card.Body>
                            <Card.Title>
                                <a href={`${explorerAddress}/explorer/transaction/${selectedTx.ID}`} target="_blank" rel="noopener noreferrer">
                                    {selectedTx.ID}
                                </a>
                            </Card.Title>
                            <ListGroup variant="flush">
                                <ListGroup.Item>Msg ID:
                                    <a href={`${explorerAddress}/explorer/messasge/${selectedTx.msgID}`} target="_blank" rel="noopener noreferrer">
                                        {selectedTx.msgID}
                                    </a>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Inputs:
                                    <ListGroup>
                                        {selectedTx.inputs.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/output/${p.referencedOutputID.base58}`} target="_blank" rel="noopener noreferrer">{p.referencedOutputID.base58}</a></ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>
                                    Outputs:
                                    <ListGroup>
                                        {selectedTx.outputs.map((p,i) => <ListGroup.Item key={i}><a href={`${explorerAddress}/explorer/output/${p}`} target="_blank" rel="noopener noreferrer">{p}</a></ListGroup.Item>)}
                                    </ListGroup>
                                </ListGroup.Item>
                                <ListGroup.Item>GoF: {selectedTx.gof}</ListGroup.Item>
                                <ListGroup.Item>Confirmed: {selectedTx.isConfirmed.toString()}</ListGroup.Item>
                                <ListGroup.Item>Confirmed Time: {dateformat(new Date(selectedTx.confirmedTime/1000000), "dd.mm.yyyy HH:MM:ss")}</ListGroup.Item>
                            </ListGroup>
                        </Card.Body>
                    </Card>
                </div>
        );
    }
}