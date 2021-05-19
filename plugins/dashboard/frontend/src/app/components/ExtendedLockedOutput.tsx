import * as React from 'react';
import {OutputID, ExtendedLockedOutput} from "app/misc/Payload";
import Badge from "react-bootstrap/Badge";
import ListGroup from "react-bootstrap/ListGroup";
import {resolveColor} from "app/utils/color";

interface Props {
    output: ExtendedLockedOutput
    id: OutputID;
}

export class ExtendedLockedOutputComponent extends React.Component<Props, any> {
    render() {
        let balances = Object.keys(this.props.output.balances).map((key) => {return {color:key, value:this.props.output.balances[key]}})
        return (
            <div className={"mb-2"} key={this.props.id.base58}>
                <ListGroup>
                    <ListGroup.Item>Type: ExtendedLockedOutput</ListGroup.Item>
                    <ListGroup.Item>
                        Balances:
                        <div>
                            {balances.map((entry, i) => (<div key={i}><Badge variant="success">{new Intl.NumberFormat().format(entry.value)} {resolveColor(entry.color)}</Badge></div>))}
                        </div>
                    </ListGroup.Item>
                    <ListGroup.Item>OutputID: <a href={`/explorer/output/${this.props.id.base58}`}>{this.props.id.base58}</a></ListGroup.Item>
                    <ListGroup.Item>Address: <a href={`/explorer/address/${this.props.output.address}`}> {this.props.output.address}</a></ListGroup.Item>
                    {
                        this.props.output.fallbackAddress &&
                        <ListGroup.Item>Fallback Address: <a href={`/explorer/address/${this.props.output.fallbackAddress}`}> {this.props.output.fallbackAddress}</a></ListGroup.Item>
                    }
                    {
                        this.props.output.fallbackDeadline &&
                        <ListGroup.Item>Fallback Deadline: {new Date(this.props.output.fallbackDeadline * 1000).toLocaleString()}</ListGroup.Item>
                    }
                    {
                        this.props.output.timelock &&
                        <ListGroup.Item>Timelocked Until: {new Date(this.props.output.timelock * 1000).toLocaleString()}</ListGroup.Item>
                    }
                    <ListGroup.Item>Transaction: <a href={`/explorer/transaction/${this.props.id.transactionID}`}> {this.props.id.transactionID}</a></ListGroup.Item>
                    <ListGroup.Item>Output Index: {this.props.id.outputIndex}</ListGroup.Item>
                </ListGroup>
            </div>
        );
    }
}



