import * as React from 'react';
import {KeyboardEvent} from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import FormControl from "react-bootstrap/FormControl";
import ChatStore from "app/stores/ChatStore";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import InputGroup from "react-bootstrap/InputGroup";

interface Props {
    nodeStore?: NodeStore;
    chatStore?: ChatStore;
}

@inject("nodeStore")
@inject("chatStore")
@observer
export class ChatSend extends React.Component<Props, any> {
    message: string;

    updateSend = (e) => {
        this.message =e.target.value;
    };

    sendMessage = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.chatStore.sendMessage(this.message);
        this.message = "";
    };

    render() {
        let {sending} = this.props.chatStore;

        return (
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                    <h6>Send a message via the Tangle</h6>
                        <InputGroup className="mb-3">
                            <FormControl
                                placeholder="Send Message"
                                aria-label="Send Message"
                                aria-describedby="basic-addon1"
                                value={this.message} onChange={this.updateSend}
                                onKeyUp={this.sendMessage}
                                disabled={sending}
                            />
                        </InputGroup>
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
