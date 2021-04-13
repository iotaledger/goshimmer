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
import {outputToComponent} from "app/utils/output";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    txId: string
}

const style = {
    maxHeight: "1000px",
    overflow: "auto",
    width: "47%",
    fontSize: "80%",
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
                                                      {outputToComponent(input.output)}
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
                                                        <span className="mb-2">Index: <Badge variant={"primary"}>{i}</Badge></span>
                                                        {outputToComponent(output)}
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