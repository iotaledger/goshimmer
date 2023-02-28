import {computed, action, observable} from "mobx";
import { registerHandler, WSMsgType } from "app/misc/WS";
import * as React from "react";
import {Link} from 'react-router-dom';

const liveFeedSize = 100;

export class SlotInfo {
    index: number;
    id: string;
}

export class SlotStore {
    @observable liveFeed: Array<SlotInfo> = [];

    constructor() {
        registerHandler(WSMsgType.SlotInfo, this.addLiveFeed);
    }

    @action addLiveFeed = async (info: SlotInfo) => {
        if (this.liveFeed.findIndex((t) => t.id == info.id) === -1) {
            if (this.liveFeed.length >= liveFeedSize) {
                this.liveFeed.shift();
            }
            this.liveFeed.push(info);
        }
    }

    @computed
    get slotLiveFeed() {
        let feed = [];
        for (let i = this.liveFeed.length - 1; i >= 0; i--) {
            let info = this.liveFeed[i];
            feed.push(
                <tr key={info.id}>
                    <td>
                        {info.index}
                    </td>
                    <td>
                        <Link to={`/explorer/slot/commitment/${info.id}`}>
                            {info.id}
                        </Link>
                    </td>
                </tr>
            );
        }
        return feed;
    }
}