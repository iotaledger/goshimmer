import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import Alert from "react-bootstrap/Alert";
import * as dateformat from 'dateformat';

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            id: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerAddressQueryResult extends React.Component<Props, any> {

    componentDidMount() {
        this.props.explorerStore.resetSearch();
        this.props.explorerStore.searchAddress(this.props.match.params.id);
    }

    getSnapshotBeforeUpdate(prevProps: Props, prevState) {
        if (prevProps.match.params.id !== this.props.match.params.id) {
            this.props.explorerStore.searchAddress(this.props.match.params.id);
        }
        return null;
    }

    render() {
        let {id} = this.props.match.params;
        let {addr, query_loading, query_err} = this.props.explorerStore;
        let outputs = [];
        let available_balances = [];
        let total_balance = new Map();

        let get_balances = function (balances) {
            if (balances.length == 0) {
                return "empty";
            }
            return balances;
         }

        if (query_err) {
            return (
                <Container>
                    <h3>Address not available - 404</h3>
                    <p>
                        Address {id} not found.
                    </p>
                </Container>
            );
        }

        if (addr) {
            for (let i = 0; i < addr.output_ids.length; i++) {
                let output = addr.output_ids[i];

                let consumed = "Spent: ";
                let conflicting = "Conflicting: false";
                if (output.consumer_count) {
                    consumed += "true";
                    if (output.consumer_count > 1) {
                        conflicting = "Conflicting: true";
                    }
                } else {
                    consumed += "false";
                }

                let status = "Status: ";
                if (output.inclusion_state.confirmed) {
                    status += ' confirmed ';
                } else if (output.inclusion_state.rejected) {
                    status += ' rejected ';
                } else {
                    status += ' pending ';
                }

                let balances = [];
                for (let j=0; j < addr.output_ids[i].balances.length; j++) {
                    let balance = addr.output_ids[i].balances[j]
                    
                    let oldBalance = 0;
                    if (total_balance.has(balance.color)) {
                        oldBalance = total_balance.get(balance.color);
                    }
                    if (addr.output_ids[i].consumer_count == 0 && addr.output_ids[i].inclusion_state.confirmed) {
                        total_balance.set(balance.color, balance.value + oldBalance);
                    }

                    balances.push(
                        <ListGroup.Item key={balance.color}>
                            <small>
                                {'Color:'} {balance.color} {' Value:'} {balance.value}
                            </small>
                        </ListGroup.Item>
                    )
                }

                outputs.push(
                    <ListGroup.Item key={output.id}>
                        <small>
                            <div>{'Output ID:'} {output.id} {' '}</div>
                            {output.solidification_time != 0 &&
                                <div>Solidification Time: {dateformat(new Date(output.solidification_time * 1000), "dd.mm.yyyy HH:MM:ss")}</div>
                            }
                            <div>{status}</div>
                            <div>{consumed}</div>
                            <div>{conflicting}</div>
                            <div>{'Balance:'} {balances}</div>   
                        </small>
                    </ListGroup.Item>
                );
            }

            total_balance.forEach((balance: number, color: string) => {
                available_balances.push(
                    <ListGroup.Item key={color}>
                            {'Color:'} {color} {' Value:'} {balance}
                    </ListGroup.Item>
                )
            });
        }
        return (
            <Container>
                <h3>Address {addr !== null && <span>({addr.output_ids.length} Outputs)</span>}</h3>
                <p>
                    {id} {' '}
                </p>
                {
                    addr !== null ?
                        <React.Fragment>
                            {
                                addr.output_ids !== null && addr.output_ids.length === 100 &&
                                <Alert variant={"warning"}>
                                    Max. 100 outputs are shown.
                                </Alert>
                            }
                             <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup>
                                        {"Available balances:"} {get_balances(available_balances)} 
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"}>
                                        {"Outputs detail:"} {outputs}
                                    </ListGroup>
                                </Col>
                            </Row>
                        </React.Fragment>
                        :
                        <Row className={"mb-3"}>
                            <Col>
                                {query_loading && <Spinner animation="border"/>}
                            </Col>
                        </Row>
                }

            </Container>
        );
    }
}
