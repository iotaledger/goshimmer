import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import {ChatLiveFeed} from "app/components/ChatLiveFeed";
import {ChatSend} from "app/components/ChatSend";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export class Chat extends React.Component<Props, any> {
    render() {
        return (
            <Container>
                <h3>Chat</h3>
                <ChatSend/>
                <ChatLiveFeed/>
            </Container>
        );
    }
}
