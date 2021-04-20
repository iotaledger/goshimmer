import * as React from 'react';
import {KeyboardEvent} from 'react';
import NodeStore from "app/stores/NodeStore";
import FaucetStore from "app/stores/FaucetStore";
import {inject, observer} from "mobx-react";
import FormControl from "react-bootstrap/FormControl";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import Button from 'react-bootstrap/Button'
import InputGroup from "react-bootstrap/InputGroup";

interface Props {
    nodeStore?: NodeStore;
    faucetStore?: FaucetStore;
}

@inject("nodeStore")
@inject("faucetStore")
@observer
export class FaucetAddressInput extends React.Component<Props, any> {

    componentWillUnmount() {
        this.props.faucetStore.reset();
    }

    updateSend = (e) => {
        switch (e.target.name) {
            case "address":
                this.props.faucetStore.updateSend(e.target.value);
                break;
            case "accessMana":
                this.props.faucetStore.updateSendAccessManaNodeID(e.target.value);
                break;
            case "consensusMana":
                this.props.faucetStore.updateSendConsensusManaNodeID(e.target.value);
        }
    };

    executeSend = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.faucetStore.sendReq();
    };

    btnExecuteSend = () => {
        this.props.faucetStore.sendReq();
    };

    render() {
        let {send_addr, query_error, sending, send_access_mana_node_id, send_consensus_mana_node_id} = this.props.faucetStore;

        return (
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <InputGroup className="mb-3">
                            <FormControl
                                name="address"
                                placeholder="Address"
                                aria-label="Address"
                                aria-describedby="basic-addon1"
                                value={send_addr} onChange={this.updateSend}
                                onKeyUp={this.executeSend}
                                disabled={sending}
                            />
                        </InputGroup>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <InputGroup className="mb-3">
                            <FormControl
                                name="accessMana"
                                placeholder="Access mana Node ID"
                                aria-label="accessMana"
                                aria-describedby="basic-addon1"
                                value={send_access_mana_node_id} onChange={this.updateSend}
                                onKeyUp={this.executeSend}
                                disabled={sending}
                            />
                        </InputGroup>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <InputGroup className="mb-3">
                            <FormControl
                                name="consensusMana"
                                placeholder="Consensus mana Node ID"
                                aria-label="consensusMana"
                                aria-describedby="basic-addon1"
                                value={send_consensus_mana_node_id} onChange={this.updateSend}
                                onKeyUp={this.executeSend}
                                disabled={sending}
                            />
                        </InputGroup>
                    </Col>
                </Row>
                <Row className={"mb-3"}>
                    <Col>
                        <Button
                            variant="primary"
                            size="sm" block
                            onClick={this.btnExecuteSend}
                            value={send_addr}
                            disabled={sending}>
                            Send
                        </Button>
                    </Col>
                </Row>
                {
                    query_error !== "" &&
                    <Row className={"mb-3"}>
                        <Col>
                            Couldn't request funds: {query_error}
                        </Col>
                    </Row>
                }
            </React.Fragment>
        );
    }
}
