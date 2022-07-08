import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';

export class ChatBlock {
    from: string;
    to: string;
    block: string;
    blockID: string;
    timestamp: string;
}

const liveFeedSize = 10;

export class ChatStore {
    // live feed
    @observable latest_blks: Array<ChatBlock> = [];
    @observable blk: ChatBlock = null;
    
    // loading
    @observable send_loading: boolean = false;
    @observable send_err: any = null;
    
    @observable block: string = "";
    @observable sending: boolean = false;

    routerStore: RouterStore;
    nodeStore: NodeStore;

    constructor(routerStore: RouterStore, nodeStore: NodeStore) {
        this.routerStore = routerStore;
        this.nodeStore = nodeStore;
        registerHandler(WSMsgType.Chat, this.addLiveFeed);
    }

    @action
    addLiveFeed = (blk: ChatBlock) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_blks.findIndex((t) => t.blockID == blk.blockID) === -1) {
            if (this.latest_blks.length >= liveFeedSize) {
                this.latest_blks.shift();
            }
            this.latest_blks.push(blk);
        }
    };

    @action
    updateSend = (block: string) => {
        this.block = block;
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

    sendBlock = async (block: string) => {
        // this.updateQueryLoading(true);
        try {
            let res = await fetch(`/api/chat`, {
                method: 'POST',
                headers: {
                  'Accept': 'application/json',
                  'Content-Type': 'application/json'
                },
                body: JSON.stringify({from: this.nodeStore.status.id, to: 'all', block: block})
            });
            if (res.status === 400) {
                this.updateSendError("Not able to send");
                return;
            }
            const blk = await res.json();
            this.updateSending(false);
            console.log(blk);
        } catch (err) {
            // this.updateQueryError(err);
            console.log(err);
            throw new Error(err);
        }
    };

    reset() {
        this.block = "";
        this.send_loading = false;
        this.sending = false;
    }

    @computed
    get blksLiveFeed() {
        let feed = [];
        for (let i = this.latest_blks.length - 1; i >= 0; i--) {
            let blk = this.latest_blks[i];
            feed.push(
                <tr key={blk.blockID}>
                    <td>
                        {blk.from}
                    </td>
                    <td>
                        {blk.block}
                    </td>
                    <td>
                        <Link to={`/explorer/block/${blk.blockID}`}>
                        {blk.blockID}
                        </Link>
                    </td>
                    <td>
                        {blk.timestamp}
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default ChatStore;
