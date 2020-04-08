import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {Link} from 'react-router-dom';
import {RouterStore} from "mobx-react-router";

export class DrngMessage {
    instanceId: number;
    round: number; 
    value: number;
    timestamp: number;
}

class Msg {
    hash: string;
    randomValue: number;
}

const liveFeedSize = 10;

export class DrngStore {
    // live feed
    @observable latest_msgs: Array<Msg> = [];

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
    addLiveFeed = (msg: Msg) => {
        // prevent duplicates (should be fast with only size 10)
        if (this.latest_msgs.findIndex((t) => t.hash == msg.hash) === -1) {
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
                <tr key={msg.hash}>
                    <td>
                        <Link to={`/drng/msg/${msg.hash}`}>
                            {msg.hash.substr(0, 35)}
                        </Link>
                    </td>
                    <td>
                        {msg.randomValue}
                    </td>
                </tr>
            );
        }
        return feed;
    }

}

export default DrngStore;