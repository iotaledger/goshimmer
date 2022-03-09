import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {ExplorerStore, ExplorerOutput, OutputMetadata} from "app/stores/ExplorerStore";
import Spinner from "react-bootstrap/Spinner";
import ListGroup from "react-bootstrap/ListGroup";
import Alert from "react-bootstrap/Alert";
import {Link} from 'react-router-dom';
import {displayManaUnit} from "app/utils";
import {outputToComponent, totalBalanceFromExplorerOutputs} from "app/utils/output";
import {Badge, Button, ListGroupItem} from "react-bootstrap";
import {resolveBase58BranchID} from "app/utils/branch";

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
        // spent outputs
        let spent: Array<ExplorerOutput> = [];
        // unspent outputs
        let unspent: Array<ExplorerOutput> = [];
        let available_balances = [];

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
            // separate spent from unspent
            addr.explorerOutputs.forEach((o) => {
                if (o.metadata.consumerCount > 0) {
                    spent.push(o);
                } else {
                    unspent.push(o);
                }
            })

            let timestampCompareFn = (a: ExplorerOutput, b: ExplorerOutput) => {
                if (b.txTimestamp === a.txTimestamp) {
                    // outputs have the same timestamp
                    if (b.id.transactionID == a.id.transactionID) {
                        // outputs belong to the same tx, sort based on index
                        return b.id.outputIndex - a.id.outputIndex;
                    }
                    // same timestamp, but different tx
                    return b.id.transactionID.localeCompare(a.id.transactionID);
                }
                return b.txTimestamp - a.txTimestamp;
            }

            // sort outputs
            unspent.sort(timestampCompareFn)
            spent.sort(timestampCompareFn)

            // derive the available funds
            totalBalanceFromExplorerOutputs(unspent, addr.address).forEach((balance: number, color: string) => {
                available_balances.push(
                    <ListGroup.Item key={color} style={{textAlign: 'center'}}>
                        <Row>
                            <Col xs={9}>
                                {color}
                            </Col>
                            <Col>
                                {new Intl.NumberFormat().format(balance)}
                            </Col>
                        </Row>
                    </ListGroup.Item>
                )
            });
        }
        return (
            <Container>
                <h3 style={{marginBottom: "40px"}}>Address <strong>{id}</strong> {addr !== null && <span>({addr.explorerOutputs.length} Outputs)</span>}</h3>
                {
                    addr !== null ?
                        <React.Fragment>
                            {
                                addr.explorerOutputs !== null && addr.explorerOutputs.length === 100 &&
                                <Alert variant={"warning"}>
                                    Max. 100 outputs are shown.
                                </Alert>
                            }
                             <Row className={"mb-3"}>
                                <Col xs={7}>
                                    <ListGroup>
                                        <h4>Available Balances</h4>
                                        {available_balances.length === 0? "There are no balances currently available." : <div>
                                            <ListGroupItem
                                                style={{textAlign: 'center'}}
                                                key={'header'}
                                            >
                                                <Row>
                                                    <Col xs={9}>
                                                        <strong>Color</strong>
                                                    </Col>
                                                    <Col>
                                                        <strong>Balance</strong>
                                                    </Col>
                                                </Row>
                                            </ListGroupItem>
                                            {available_balances}
                                        </div> }
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"}>
                                        <h4>Unspent Outputs</h4>
                                        {unspent.length === 0? "There are no unspent outputs currently available." : <div>
                                            {unspent.map((o) => {
                                                return <OutputButton output={o}/>
                                            })}
                                        </div>
                                        }
                                    </ListGroup>
                                </Col>
                            </Row>
                            <Row className={"mb-3"}>
                                <Col>
                                    <ListGroup variant={"flush"}>
                                        <h4>Spent Outputs</h4>
                                        {spent.length === 0? "There are no spent outputs currently available." : <div>
                                            {spent.map((o) => {
                                                return <OutputButton output={o}/>
                                            })}
                                        </div>
                                        }
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

interface oProps {
    output: ExplorerOutput;
}

class OutputButton extends React.Component<oProps, any> {
    constructor(props) {
        super(props);
        this.state = {
            enabled: false
        };
    }

    render() {
        return (
            <ListGroup.Item>
                <Button
                    variant={getVariant(this.props.output.output.type)}
                    onClick={ () => { this.setState({enabled: !this.state.enabled})}}
                    block
                >
                 <Row>
                     <Col xs={6} style={{textAlign: "left"}}>{this.props.output.id.base58} </Col>
                     <Col style={{textAlign: "left"}}>{this.props.output.output.type.replace("Type", "")} </Col>
                     <Col style={{textAlign: "left"}}>{new Date(this.props.output.txTimestamp * 1000).toLocaleString()}</Col>
                 </Row>
                </Button>
                <Row style={{fontSize: "90%"}}>
                    <Col>
                        {
                            this.state.enabled? outputToComponent(this.props.output.output): null
                        }
                    </Col>
                    <Col>
                        {
                            this.state.enabled? <OutputMeta
                                metadata={this.props.output.metadata}
                                timestamp={this.props.output.txTimestamp}
                                pendingMana={this.props.output.pendingMana}
                            />: null
                        }
                    </Col>
                </Row>
            </ListGroup.Item>
            );
    }
}

interface omProps {
    metadata: OutputMetadata;
    timestamp: number;
    pendingMana: number;
}

class OutputMeta extends React.Component<omProps, any> {
    render() {
        let metadata = this.props.metadata;
        let timestamp = this.props.timestamp;
        let pendingMana = this.props.pendingMana;
        return (
            <ListGroup>
                <ListGroup.Item>Grade of Finality: {deriveSolid(metadata)} {metadata.gradeOfFinality}</ListGroup.Item>
                BranchIDs: 
                <ListGroup>
                    {
                        metadata.branchIDs.map((value, index) => {
                            return (
                                <ListGroup.Item key={"BranchID" + index + 1} className="text-break">
                                    <Link to={`/explorer/branch/${value}`}>
                                        {resolveBase58BranchID(value)}
                                    </Link>
                                </ListGroup.Item>
                            )
                        })
                    }
                </ListGroup>
                <ListGroup.Item>Pending mana: {displayManaUnit(pendingMana)}</ListGroup.Item>
                <ListGroup.Item>Timestamp: {new Date(timestamp * 1000).toLocaleString()}</ListGroup.Item>
                <ListGroup.Item>Solidification Time: {new Date(metadata.solidificationTime * 1000).toLocaleString()}</ListGroup.Item>
                <ListGroup.Item>Consumer Count: {metadata.consumerCount}</ListGroup.Item>
                { metadata.confirmedConsumer && <ListGroup.Item>Confirmed Consumer: <a href={`/explorer/transaction/${metadata.confirmedConsumer}`}>{metadata.confirmedConsumer}</a> </ListGroup.Item>}
            </ListGroup>
        );
    }
}

let deriveSolid = (m: OutputMetadata) => {
    return m.solid? <Badge variant={"success"}>solid</Badge>: <Badge variant={"danger"}>not solid</Badge>;
}

let getVariant = (outputType) => {
    switch (outputType) {
        case "SigLockedSingleOutputType":
            return "light";
        case "SigLockedColoredOutputType":
            return "light";
        case "AliasOutputType":
            return "success";
        case "ExtendedLockedOutputType":
            return "info";
        default:
            return "danger";
    }
}