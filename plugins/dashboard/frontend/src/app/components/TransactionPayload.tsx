import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Badge from "react-bootstrap/Badge"
import ListGroup from "react-bootstrap/ListGroup";
import {Link} from 'react-router-dom';
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
import {IconContext} from "react-icons"
import {FaChevronCircleRight} from "react-icons/fa"

interface Props {
    explorerStore?: ExplorerStore;
}

interface OutputProps {
    address: string;
    outputID: string;
    balances?: Array<BalanceProps>
    index: number;
}

interface UnlockProps {
    signature: string;
    index: number;
}

interface BalanceProps {
    value: number;
    color: string;
}

@inject("explorerStore")
@observer
export class TransactionPayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;

        return (
            payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Transaction ID: <a href={`/explorer/transaction/${payload.tx_id}`}>{payload.tx_id}</a></ListGroup.Item>
                        </ListGroup>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Timestamp: {new Date(payload.tx_essence.timestamp*1000).toLocaleString()} </ListGroup.Item>
                            <ListGroup.Item>Access Pledge ID: {payload.tx_essence.access_pledge_id}</ListGroup.Item>
                            <ListGroup.Item>Consensus Pledge ID: {payload.tx_essence.cons_pledge_id}</ListGroup.Item>
                        </ListGroup>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <div style={{
                            marginBottom: "20px",
                            paddingBottom: "10px",
                            borderBottom: "2px solid #eee"}}>Inputs:</div>
                        <React.Fragment>
                            {
                                payload.tx_essence.inputs.map((input,index) => (
                                    <Inputs
                                        address={input.address}
                                        outputID={input.output_id}
                                        balances={input.balance}
                                        index={index}
                                        key={input.address}
                                    />
                            ))}
                        </React.Fragment>
                    </Col>
                    <Col md="auto">
                        <IconContext.Provider value={{ color: "#00a0ff", size: "2em"}}>
                            <div style={{
                                marginTop: "40px"}}>
                                <FaChevronCircleRight />
                            </div>
                        </IconContext.Provider>
                    </Col>
                    <Col>
                        <div style={{
                            marginBottom: "20px",
                            paddingBottom: "10px",
                            borderBottom: "2px solid #eee"}}>Outputs:</div>
                        <React.Fragment>
                            {
                                payload.tx_essence.outputs.map((output,index) => (
                                    <Outputs
                                        address={output.address}
                                        outputID={output.output_id}
                                        balances={output.balance}
                                        index={index}
                                        key={output.address}
                                    />
                            ))}
                        </React.Fragment>
                    </Col>
                </Row>
                { 
                    payload.tx_essence.data &&
                    <Row className={"mb-3"}>
                        <Col>
                            <ListGroup>
                                <ListGroup.Item>Data: {payload.tx_essence.data} </ListGroup.Item>
                            </ListGroup>
                        </Col>
                    </Row>
                }
                <Row className={"mb-3"}>
                    <Col>
                        <div style={{
                            marginBottom: "20px",
                            paddingBottom: "10px",
                            borderBottom: "2px solid #eee"}}>Unlock Blocks:</div>
                        <React.Fragment>
                            {
                                payload.unlock_blocks.map((block,index) => (
                                    <UnlockBlock
                                        signature={block}
                                        index={index}
                                        key={index}
                                    />
                                ))}
                        </React.Fragment>
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}

class Inputs extends React.Component<OutputProps> {
    render() {
        return (
            <Row className={"mb-3"}>
                <Col>
                    Index: <Badge variant={"primary"}>{this.props.index}</Badge>
                    <ListGroup>
                        <ListGroup.Item key={this.props.outputID}>OutputID: <a href={`/explorer/output/${this.props.outputID}`}>{this.props.outputID}</a> </ListGroup.Item>
                        <ListGroup.Item key={this.props.address}>Address: <Link to={`/explorer/address/${this.props.address}`}>{this.props.address}</Link>
                        </ListGroup.Item>
                        <ListGroup.Item key={'balances'}>
                            Balances:
                            <div>
                                {this.props.balances.map( balance => (<div key={balance.color}><Badge variant="danger">{balance.value} {balance.color}</Badge></div>))}
                            </div>
                        </ListGroup.Item>
                    </ListGroup>
                </Col>
            </Row>
        );
    }
}

class Outputs extends React.Component<OutputProps> {
    render() {
        return (
            <Row className={"mb-3"}>
                <Col>
                    Index: <Badge variant={"primary"}>{this.props.index}</Badge>
                    <ListGroup>
                        <ListGroup.Item key={this.props.outputID}>OutputID: <a href={`/explorer/output/${this.props.outputID}`}>{this.props.outputID}</a></ListGroup.Item>
                        <ListGroup.Item key={this.props.address}>Address: <Link to={`/explorer/address/${this.props.address}`}>{this.props.address}</Link>
                        </ListGroup.Item>
                        <ListGroup.Item key={'balances'}>
                            Balances:
                            <div>
                                {this.props.balances.map( balance => (<div key={balance.color}><Badge variant="success">{balance.value} {balance.color}</Badge></div>))}
                            </div>
                        </ListGroup.Item>
                    </ListGroup>
                </Col>
            </Row>
        );
    }
}

class UnlockBlock extends React.Component<UnlockProps> {
    render() {
        return (
            <Row className={"mb-3"}>
                <Col>
                    Index: <Badge variant={"primary"}>{this.props.index}</Badge>
                    <ListGroup>
                        <ListGroup.Item key={this.props.index}>Signature: {this.props.signature}</ListGroup.Item>
                    </ListGroup>
                </Col>
            </Row>
        );
    }
}
