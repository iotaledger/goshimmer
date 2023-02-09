import {computed, action, observable} from "mobx";
import { registerHandler, WSMsgType } from "app/misc/WS";
import * as React from "react";
import {Link} from 'react-router-dom';

const liveFeedSize = 100;

export class EpochInfo {
    index: number;
    id: string;
}

export class EpochStore {
    @observable liveFeed: Array<EpochInfo> = [];

    constructor() {
        registerHandler(WSMsgType.EpochInfo, this.addLiveFeed);
    }

    @action addLiveFeed = async (info: EpochInfo) => {
        if (this.liveFeed.findIndex((t) => t.id == info.id) === -1) {
            if (this.liveFeed.length >= liveFeedSize) {
                this.liveFeed.shift();
            }
            this.liveFeed.push(info);
        }
    }

    @computed
    get epochLiveFeed() {
        let feed = [];
        for (let i = this.liveFeed.length - 1; i >= 0; i--) {
            let info = this.liveFeed[i];
            feed.push(
                <tr key={info.id}>
                    <td>
                        {info.index}
                    </td>
                    <td>
                        <Link to={`/explorer/epoch/${info.id}`}>
                            {info.id}
                        </Link>
                    </td>
                </tr>
            );
        }
        return feed;
    }
}