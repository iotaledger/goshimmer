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
    balances?: BalanceProps
}

interface BalanceProps {
    value: number;
    color: string;
}

@inject("explorerStore")
@observer
export class ValuePayload extends React.Component<Props, any> {

    render() {
        let {payload} = this.props.explorerStore;

        return (
            payload &&
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Parent 1: {payload.parent1_id} </ListGroup.Item>
                        </ListGroup>
                    </Col>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Parent 2: {payload.parent2_id} </ListGroup.Item>
                        </ListGroup>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <ListGroup>
                            <ListGroup.Item>Transaction ID: {payload.tx_id} </ListGroup.Item>
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
                                payload.inputs.map(input => (
                                    <Inputs address={input.address} key={input.address}/>
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
                                payload.outputs.map(output => (
                                    output.balance.map(balance => (
                                        <Outputs key={balance.value} 
                                                 address={output.address}
                                                 balances={balance} />
                                    ))
                            ))}
                        </React.Fragment>
                    </Col>
                </Row>
                { 
                    payload.data &&
                    <Row className={"mb-3"}>
                        <Col>
                            <ListGroup>
                                <ListGroup.Item>Data: {payload.data} </ListGroup.Item>
                            </ListGroup>
                        </Col>
                    </Row>
                }
            </React.Fragment>
        );
    }
}

class Inputs extends React.Component<OutputProps> {
    render() {
        return (
            <Row className={"mb-3"}>
                <Col>
                    <div>
                        <Link to={`/explorer/address/${this.props.address}`}>
                            {this.props.address}
                        </Link>
                    </div>
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
                    <div>
                        <Link to={`/explorer/address/${this.props.address}`}>
                            {this.props.address}
                        </Link>
                    </div>
                    <div>
                        <Badge variant="success">
                             {this.props.balances.value}{' '}{this.props.balances.color}
                        </Badge>
                    </div>
                </Col>
            </Row>
        );
    }
}
