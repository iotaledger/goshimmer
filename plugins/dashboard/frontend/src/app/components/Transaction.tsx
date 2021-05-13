import * as React from "react";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import ListGroup from "react-bootstrap/ListGroup";
import Badge from "react-bootstrap/Badge";
import {outputToComponent} from "app/utils/output";
import {IconContext} from "react-icons";
import {FaChevronCircleRight} from "react-icons/fa";
import {UnlockBlock} from "app/components/UnlockBlock";
import {Transaction as TransactionJSON} from "app/misc/Payload";

const style = {
    maxHeight: "1000px",
    overflow: "auto",
    width: "47%",
    fontSize: "85%",
}

interface Props {
    txID?: string;
    tx?: TransactionJSON;
}

export class Transaction extends React.Component<Props, any> {
    render() {
        let txID = this.props.txID;
        let tx = this.props.tx;
        return (
            tx && txID &&
            <div>
                <h4>Transaction</h4>
                <p> {txID} </p>
                <Row className={"mb-3"}>
                    <Col>
                        <div style={{
                            marginTop: "10px",
                            marginBottom: "20px",
                            paddingBottom: "10px",
                            borderBottom: "2px solid #eee"}}><h5>Transaction Essence</h5></div>
                        <ListGroup>
                            <ListGroup.Item>ID: <a href={`/explorer/transaction/${txID}`}> {txID}</a></ListGroup.Item>
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
                <Row className={"mb-3"}>
                    <Col>
                        <div style={{
                            marginBottom: "20px",
                            paddingBottom: "10px",
                            borderBottom: "2px solid #eee"}}><h5>Unlock Blocks</h5></div>
                        <React.Fragment>
                            {
                                tx.unlockBlocks.map((block,index) => (
                                    <UnlockBlock
                                        block={block}
                                        index={index}
                                        key={index}
                                    />
                                ))}
                        </React.Fragment>
                    </Col>
                </Row>
            </div>
        );
    }
}