import * as React from 'react';
import {OutputID, SigLockedSingleOutput} from "app/misc/Payload";
import Badge from "react-bootstrap/Badge";
import ListGroup from "react-bootstrap/ListGroup";

interface Props {
    output: SigLockedSingleOutput
    id: OutputID;
}

export class SigLockedSingleOutputComponent extends React.Component<Props, any> {
    render() {
        let o = this.props.output;
        let id = this.props.id;
        return (
            <div className={"mb-2"} key={this.props.id.base58}>
                <ListGroup>
                    <ListGroup.Item>Type: SigLockedSingleOutput</ListGroup.Item>
                    <ListGroup.Item>
                        Balances:
                        <div>
                            <div><Badge variant="success">{new Intl.NumberFormat().format(o.balance)} IOTA</Badge></div>
                        </div>
                    </ListGroup.Item>
                    <ListGroup.Item>OutputID: <a href={`/explorer/output/${id.base58}`}>{id.base58}</a></ListGroup.Item>
                    <ListGroup.Item>Address: <a href={`/explorer/address/${o.address}`}> {o.address}</a></ListGroup.Item>
                <ListGroup.Item>Transaction: <a href={`/explorer/transaction/${id.transactionID}`}> {id.transactionID}</a></ListGroup.Item>
                <ListGroup.Item>Output Index: {id.outputIndex}</ListGroup.Item>
                </ListGroup>
            </div>
        );
    }
}