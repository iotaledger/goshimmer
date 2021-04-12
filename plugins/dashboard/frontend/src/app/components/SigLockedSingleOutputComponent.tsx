import * as React from 'react';
import {OutputID, SigLockedSingleOutput} from "app/misc/Payload";
import Badge from "react-bootstrap/Badge";
import ListGroup from "react-bootstrap/ListGroup";

interface Props {
    output: SigLockedSingleOutput
    id: OutputID;
    index?: number;
}

export class SigLockedSingleOutputComponent extends React.Component<Props, any> {
    render() {
        return (
            <div className={"mb-2"} key={this.props.index}>
                <span className={"mb-2"}>Index: <Badge variant={"primary"}>{this.props.index}</Badge></span>
                <ListGroup>
                    <ListGroup.Item>ID: <a href={`/explorer/output/${this.props.id.base58}`}>{this.props.id.base58}</a></ListGroup.Item>
                    <ListGroup.Item>Address: <a href={`/explorer/address/${this.props.output.address}`}> {this.props.output.address}</a></ListGroup.Item>
                    <ListGroup.Item>Type: SigLockedSingleOutput</ListGroup.Item>
                    <ListGroup.Item>Output Index: {this.props.id.outputIndex}</ListGroup.Item>
                    <ListGroup.Item>
                        Balances:
                        <div>
                            <div><Badge variant="success">ColorIOTA {this.props.output.balance}</Badge></div>
                        </div>
                    </ListGroup.Item>
                </ListGroup>
            </div>
        );
    }
}