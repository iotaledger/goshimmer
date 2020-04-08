import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore} from "mobx-react-router";

export class DrngMessage {
    instance: number;
    dpk: string;
    round: number; 
    randomness: string;
    timestamp: string;
}

const liveFeedSize = 10;

export class DrngStore {
    // live feed
    @observable latest_msgs: Array<DrngMessage> = [];

    // queries
    @observable msg: DrngMessage = null;
 
    // loading
    @observable query_loading: boolean = false;
    @observable query_err: any = null;

    routerStore: RouterStore;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
        registerHandler(WSMsgType.Drng, this.addLiveFeed);
    }

    @action
    addLiveFeed = (msg: DrngMessage) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_msgs.findIndex((t) => t.round == msg.round) === -1) {
            if (this.latest_msgs.length >= liveFeedSize) {
                this.latest_msgs.shift();
            }
            this.latest_msgs.push(msg);
        }
    };

    @computed
    get msgsLiveFeed() {
        let feed = [];
        for (let i = this.latest_msgs.length - 1; i >= 0; i--) {
            let msg = this.latest_msgs[i];
            feed.push(
                <tr key={msg.round}>
                    <td>
                        {msg.instance}
                    </td>
                    <td>
                        {msg.round}
                    </td>
                    <td>
                        {msg.randomness}
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

export default DrngStore;