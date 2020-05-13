import * as React from 'react';
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
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
                        <React.Fragment>
                            <Row className={"mb-3"}>
                                <Col>
                                    <div style={{
                                        marginBottom: "20px",
                                        paddingBottom: "10px",
                                        borderBottom: "2px solid #eee"}}>Inputs:</div>
                                    <Row className={"mb-3"}>
                                        <Col>
                                            {payload.inputs.map(input => (
                                                <Inputs address={input.address} key={input.address}/>
                                            ))}
                                        </Col>
                                    </Row>
                                </Col>
                                <Col md="auto">
                                    <FaChevronCircleRight />
                                </Col>
                                <Col>
                                    <div style={{
                                        marginBottom: "20px",
                                        paddingBottom: "10px",
                                        borderBottom: "2px solid #eee"}}>Outputs:</div>
                                    <Row className={"mb-3"}>
                                        <Col>
                                            {payload.outputs.map(output => (
                                                output.balance.map(balance => (
                                                    <Outputs key={output.address} 
                                                             address={output.address}
                                                             balances={balance} />
                                                ))
                                            ))}
                                        </Col>
                                    </Row>
                                </Col>
                            </Row>
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
            <p>{this.props.address}</p>
        );
    }
}

class Outputs extends React.Component<OutputProps> {
    render() {
        return (
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <div>
                            <span>{this.props.address}</span>
                        </div>
                        <div>
                            <span>{this.props.balances.color}: {this.props.balances.value}</span>
                        </div>
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
