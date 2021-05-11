import * as React from 'react';
import {AliasOutput, OutputID} from "app/misc/Payload";
import Badge from "react-bootstrap/Badge";
import ListGroup from "react-bootstrap/ListGroup";
import {resolveColor} from "app/utils/color";

interface Props {
    output: AliasOutput
    id: OutputID;
}

export class AliasOutputComponent extends React.Component<Props, any> {
    render() {
        let balances = Object.keys(this.props.output.balances).map((key) => {return {color:key, value:this.props.output.balances[key]}})
        return (
            <div className={"mb-2"} key={this.props.id.base58}>
                <ListGroup style={{wordBreak: "break-word"}}>
                    <ListGroup.Item>Type: AliasOutput</ListGroup.Item>
                    <ListGroup.Item>
                        Balances:
                        <div>
                            {balances.map((entry, i) => (<div key={i}><Badge variant="success">{new Intl.NumberFormat().format(entry.value)} {resolveColor(entry.color)}</Badge></div>))}
                        </div>
                    </ListGroup.Item>
                    <ListGroup.Item>OutputID: <a href={`/explorer/output/${this.props.id.base58}`}>{this.props.id.base58}</a></ListGroup.Item>
                    <ListGroup.Item>AliasAddress: <a href={`/explorer/address/${this.props.output.aliasAddress}`}> {this.props.output.aliasAddress}</a></ListGroup.Item>
                    <ListGroup.Item>StateAddress: <a href={`/explorer/address/${this.props.output.stateAddress}`}> {this.props.output.stateAddress}</a></ListGroup.Item>
                    <ListGroup.Item>Governing Address:  {this.props.output.governingAddress?  <a href={`/explorer/address/${this.props.output.governingAddress}`}> {this.props.output.governingAddress}</a> : "Self-governed"} </ListGroup.Item>
                    <ListGroup.Item>State Index: {this.props.output.stateIndex}</ListGroup.Item>
                    {
                        this.props.output.stateData &&
                        <ListGroup.Item>State Data: {this.props.output.stateData}</ListGroup.Item>
                    }
                    {
                        this.props.output.governanceMetadata &&
                        <ListGroup.Item>Governance Metadata: {this.props.output.governanceMetadata}</ListGroup.Item>
                    }
                    {
                        this.props.output.immutableData &&
                        <ListGroup.Item>Immutable Data: {this.props.output.immutableData}</ListGroup.Item>
                    }
                    <ListGroup.Item>Governance Update: {this.props.output.isGovernanceUpdate.toString()}</ListGroup.Item>
                    <ListGroup.Item>Origin: {this.props.output.isOrigin.toString()}</ListGroup.Item>
                    <ListGroup.Item>Delegated: {this.props.output.isDelegated.toString()}</ListGroup.Item>
                    {
                        this.props.output.delegationTimelock &&
                        <ListGroup.Item>Delegation Timelocked Until: {new Date(this.props.output.delegationTimelock * 1000).toLocaleString()}</ListGroup.Item>
                    }
                    <ListGroup.Item>Transaction: <a href={`/explorer/transaction/${this.props.id.transactionID}`}> {this.props.id.transactionID}</a></ListGroup.Item>
                    <ListGroup.Item>Output Index: {this.props.id.outputIndex}</ListGroup.Item>
                </ListGroup>
            </div>
        );
    }
}
