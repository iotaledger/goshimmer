import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';

export class ChatMessage {
    from: string;
    to: string;
    message: string;
    messageID: string;
    timestamp: string;
}

const liveFeedSize = 10;

export class ChatStore {
    // live feed
    @observable latest_msgs: Array<ChatMessage> = [];
    @observable msg: ChatMessage = null;
    
    // loading
    @observable send_loading: boolean = false;
    @observable send_err: any = null;
    
    @observable message: string = "";
    @observable sending: boolean = false;

    routerStore: RouterStore;
    nodeStore: NodeStore;

    constructor(routerStore: RouterStore, nodeStore: NodeStore) {
        this.routerStore = routerStore;
        this.nodeStore = nodeStore;
        registerHandler(WSMsgType.Chat, this.addLiveFeed);
    }

    @action
    addLiveFeed = (msg: ChatMessage) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_msgs.findIndex((t) => t.messageID == msg.messageID) === -1) {
            if (this.latest_msgs.length >= liveFeedSize) {
                this.latest_msgs.shift();
            }
            this.latest_msgs.push(msg);
        }
    };

    @action
    updateSend = (message: string) => {
        this.message = message;
    };

    @action
    updateSending = (sending: boolean) => this.sending = sending;

    @action
    updateSendLoading = (loading: boolean) => this.send_loading = loading;

    @action
    updateSendError = (err: any) => {
        this.send_err = err;
        this.send_loading = false;
        this.sending = false;
    };

    sendMessage = async (message: string) => {
        // this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/chat`, {
                method: 'POST',
                headers: {
                  'Accept': 'application/json',
                  'Content-Type': 'application/json'
                },
                body: JSON.stringify({from: this.nodeStore.status.id, to: 'all', message: message})
            });
            if (res.status === 400) {
                this.updateSendError("Not able to send");
                return;
            }
            const msg = await res.json();
            this.updateSending(false);
            console.log(msg);
        } catch (err) {
            // this.updateQueryError(err);
            console.log(err);
            throw new Error(err);
        }
    };

    reset() {
        this.message = "";
        this.send_loading = false;
        this.sending = false;
    }

    @computed
    get msgsLiveFeed() {
        let feed = [];
        for (let i = this.latest_msgs.length - 1; i >= 0; i--) {
            let msg = this.latest_msgs[i];
            feed.push(
                <tr key={msg.messageID}>
                    <td>
                        {msg.from}
                    </td>
                    <td>
                        {msg.message}
                    </td>
                    <td>
                        <Link to={`/explorer/message/${msg.messageID}`}>
                        {msg.messageID}
                        </Link>
                    </td>
                    <td>
                        {msg.timestamp}
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default ChatStore;