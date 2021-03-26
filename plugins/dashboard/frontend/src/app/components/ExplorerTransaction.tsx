import * as React from 'react';
import Container from "react-bootstrap/Container";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import {FaChevronCircleRight} from "react-icons/fa";
import {IconContext} from "react-icons";
import Badge from "react-bootstrap/Badge";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    txId: string
}

const style = {
    maxHeight: "500px",
    overflow: "auto",
    width: "47%",
    fontSize: "90%",
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerTransaction extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getTransaction(this.props.txId);
    }
    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let { txId } = this.props;
        let { query_err, tx } = this.props.explorerStore;

        if (query_err) {
            return (
                <Container>
                    <h3>Transaction not available - 404</h3>
                    <p>
                        Transaction with ID {txId} not found.
                    </p>
                </Container>
            );
        }
        return (
            <Container>
                <h3>Transaction</h3>
                <p> {txId} </p>


                {tx &&
                    <Row className={"mb-3"}>
                        <Col>
                            <ListGroup>
                                <ListGroup.Item>ID: {txId}</ListGroup.Item>
                                <ListGroup.Item>Version: {tx.version}</ListGroup.Item>
                                <ListGroup.Item>Timestamp: {new Date(tx.timestamp * 1000).toLocaleString()}</ListGroup.Item>
                                <ListGroup.Item>Access pledge ID: {tx.accessPledgeID}</ListGroup.Item>
                                <ListGroup.Item>Consensus pledge ID: {tx.consensusPledgeID}</ListGroup.Item>
                                <ListGroup.Item>
                                    <div className="d-flex justify-content-between align-items-center">
                                      <div className="align-self-start input-output-list" style={style}>
                                          <span>Inputs</span>
                                          <hr/>
                                          {tx.inputs.map((input, i) => {
                                              return (
                                                  <div className={"mb-2"} key={i}>
                                                      <span className="mb-2">Index: <Badge variant={"primary"}>{i}</Badge></span>
                                                      <ListGroup className={"mb-1"}>
                                                          {
                                                              input.referencedOutputID ?
                                                                  <React.Fragment>
                                                                      <ListGroup.Item>Transaction ID: <a href={`/explorer/transaction/${input.referencedOutputID.transactionID}`}>{input.referencedOutputID.transactionID}</a></ListGroup.Item>
                                                                      <ListGroup.Item>Referenced OutputID: <a href={`/explorer/output/${input.referencedOutputID.base58}`}>{input.referencedOutputID.base58}</a></ListGroup.Item>
                                                                      <ListGroup.Item>Output Index: {input.referencedOutputID.outputIndex}</ListGroup.Item>
                                                                  </React.Fragment>
                                                              :
                                                                 <React.Fragment>
                                                                     <ListGroup.Item>Transaction ID: Genesis</ListGroup.Item>
                                                                     <ListGroup.Item>Referenced OutputID: Genesis</ListGroup.Item>
                                                                     <ListGroup.Item>Output Index: Genesis</ListGroup.Item>
                                                                 </React.Fragment>
                                                          }

                                                          <ListGroup.Item>Type: {input.type}</ListGroup.Item>
                                                          {input.referencedOutput && <ListGroup.Item>
                                                              Balances:
                                                              <div>
                                                                  {Object.entries(input.referencedOutput.balances).map((entry, i) => (<div key={i}><Badge variant="danger">{entry[1]} {entry[0]}</Badge></div>))}
                                                              </div>
                                                          </ListGroup.Item>}
                                                      </ListGroup>
                                                  </div>
                                              )
                                          })}
                                      </div>
                                          <IconContext.Provider value={{ color: "#00a0ff", size: "2em"}}>
                                              <div>
                                                  <FaChevronCircleRight />
                                              </div>
                                          </IconContext.Provider>
                                      <div style={style}>
                                            <span>Outputs</span>
                                            <hr/>
                                            {tx.outputs.map((output, i) => {
                                                return (
                                                    <div className={"mb-2"} key={i}>
                                                        <span className={"mb-2"}>Index: <Badge variant={"primary"}>{i}</Badge></span>
                                                        <ListGroup>
                                                            <ListGroup.Item>ID: <a href={`/explorer/output/${output.outputID.base58}`}>{output.outputID.base58}</a></ListGroup.Item>
                                                            <ListGroup.Item>Address: <a href={`/explorer/address/${output.address}`}> {output.address}</a></ListGroup.Item>
                                                            <ListGroup.Item>Type: {output.type}</ListGroup.Item>
                                                            <ListGroup.Item>Output Index: {output.outputID.outputIndex}</ListGroup.Item>
                                                            <ListGroup.Item>
                                                                Balances:
                                                                <div>
                                                                    {Object.entries(output.balances).map((entry, i) => (<div key={i}><Badge variant="success">{entry[1]} {entry[0]}</Badge></div>))}
                                                                </div>
                                                            </ListGroup.Item>
                                                        </ListGroup>
                                                    </div>
                                                )
                                            })}
                                        </div>
                                    </div>
                                </ListGroup.Item>
                                { tx.dataPayload && <ListGroup.Item>Data payload: {tx.dataPayload}</ListGroup.Item>}
                            </ListGroup>
                        </Col>
                    </Row>
                }
            </Container>
        )
    }
}