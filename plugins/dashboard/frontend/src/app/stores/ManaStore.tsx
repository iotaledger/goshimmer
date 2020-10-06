import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";

class ManaMsg {
    nodeID: string;
    access: number;
    consensus: number;
    // in s?
    time: number;
}

class Node {
    nodeID: string;
    mana: number;
}

class NetworkManaMsg {
    manaType: string;
    totalMana: number;
    nodes: Array<Node>;
}

export class AllowedPledgeIDsMsg {
    accessFilter: PledgeIDFilter;
    consensusFilter: PledgeIDFilter;
}

export class PledgeIDFilter {
    enabled: boolean;
    allowedNodeIDs: Array<AllowedNodeStr>;
}

export class AllowedNodeStr {
    shortID: string;
    fullID: string;
}

// every 10 seconds, a new value arrives, so this is roughly 166 mins
const maxStoredManaValues = 1000;

export class ManaStore {
    // mana values
    @observable manaValues: Array<any> = [];
    // first is accessm second consensus
    @observable prevManaValues: Array<number> = [0,0];
    // list of richest access mana nodes in  network, sorted in descending order
    @observable accessNetworkRichest: Array<Node> = [];
    @observable totalAccessNetwork: number = 0.0;
    // list of richest active access mana nodes in the network, sorted in descending order
    @observable accessActiveRichest: Array<Node> = [];
    @observable totalAccessActive: number = 0.0;
    // list of richest consensus mana nodes in their network, sorted in descending order
    @observable consensusNetworkRichest: Array<Node> = [];
    @observable totalConsensusNetwork: number = 0.0;
    // list of richest active consensus mana nodes in their network, sorted in descending order
    @observable consensusActiveRichest: Array<Node> = [];
    @observable totalConsensusActive: number = 0.0;

    @observable public searchNode = "";

    @observable public allowedPledgeIDs: AllowedPledgeIDsMsg;

    ownID: string;

    constructor() {
        this.manaValues = [];
        registerHandler(WSMsgType.Mana, this.addNewManaValue);
        registerHandler(WSMsgType.ManaMapOverall, this.updateNetworkRichest);
        registerHandler(WSMsgType.ManaMapOnline, this.updateActiveRichest);
        registerHandler(WSMsgType.ManaAllowedPledge, this.updateAllowedPledgeIDs);
    };

    @action
    updateSearch(searchNode: string): void {
        this.searchNode = searchNode.trim();
    }

    @action
    addNewManaValue = (manaMsg: ManaMsg) =>  {
        this.ownID = this.ownID? this.ownID : manaMsg.nodeID;
        if (this.manaValues.length === maxStoredManaValues) {
            // shift if we already have enough values
            this.manaValues.shift();
        }
        let newManaData = [new Date(manaMsg.time*1000), manaMsg.access, manaMsg.consensus];
        if (this.manaValues.length > 0){
            this.prevManaValues = [this.manaValues[this.manaValues.length -1][1] , this.manaValues[this.manaValues.length -1][2]]
        }
        this.manaValues.push(newManaData);
    }

    @action
    updateNetworkRichest = (msg: NetworkManaMsg) => {
        let tmp = msg;
        console.log(tmp);
        switch (msg.manaType) {
            case "Access Mana":
                this.totalAccessNetwork = msg.totalMana;
                this.accessNetworkRichest = msg.nodes;
                break;
            case "Consensus Mana":
                this.totalConsensusNetwork = msg.totalMana;
                this.consensusNetworkRichest = msg.nodes;
                break;

        }
    }

    @action
    updateActiveRichest = (msg: NetworkManaMsg) => {
        switch (msg.manaType) {
            case "Access Mana":
                this.totalAccessActive = msg.totalMana;
                this.accessActiveRichest = msg.nodes;
                break;
            case "Consensus Mana":
                this.totalConsensusActive = msg.totalMana;
                this.consensusActiveRichest = msg.nodes;
                break;

        }
    };

    @action
    updateAllowedPledgeIDs = (msg: AllowedPledgeIDsMsg) => {
        this.allowedPledgeIDs = msg;
    }

    nodeList = (leaderBoard: Array<Node>, manaSum: number) => {
        if (leaderBoard === null || undefined) {
            return []
        }
        let feed = []
        let pushToFeed = (node: Node, i: number) => {
            feed.push(
                <tr
                    key={node.nodeID}
                    style={{
                        color: node.nodeID === this.ownID? 'white': 'black',
                        backgroundColor: node.nodeID === this.ownID ? 'rgba(7, 90, 184, 0.8)': 'white',
                    }}
                >
                    <td style={
                        {
                            borderTopLeftRadius: node.nodeID === this.ownID ? '10px': '0',
                            borderBottomLeftRadius: node.nodeID === this.ownID ? '10px': '0',
                        }
                    }> {i + 1} </td>
                    <td>{node.nodeID}</td>
                    <td>{node.mana.toFixed(2)}</td>
                    <td style={
                        {
                            borderTopRightRadius: node.nodeID === this.ownID ? '10px': '0',
                            borderBottomRightRadius: node.nodeID === this.ownID ? '10px': '0',
                        }
                    }>{((node.mana / manaSum)*100.0).toFixed(2)}%</td>
                </tr>
            );
        };
        let callbackNoSearch = (node: Node, i: number) => {
            pushToFeed(node, i);
        };
        let callbackSearch = (node: Node, i: number) => {
            if (node.nodeID.toLowerCase().includes(this.searchNode.toLowerCase())){
                pushToFeed(node, i);
            }
        };
        if (this.searchNode.trim().length === 0) {
            leaderBoard.forEach(callbackNoSearch);
        } else {
            leaderBoard.forEach(callbackSearch)
        }
        return feed
    }

    @computed
    get networkRichestFeedAccess() {
        return this.nodeList(this.accessNetworkRichest, this.totalAccessNetwork);
    }

    @computed
    get networkRichestFeedConsensus() {
        return this.nodeList(this.consensusNetworkRichest, this.totalConsensusNetwork);
    }

    @computed
    get activeRichestFeedAccess() {
        return this.nodeList(this.accessActiveRichest, this.totalAccessActive);
    }

    @computed
    get activeRichestFeedConsensus() {
        return this.nodeList(this.consensusActiveRichest, this.totalConsensusActive);
    }

    @computed
    get accessHistogramInput() {
        if (this.accessNetworkRichest === undefined || this.accessNetworkRichest === null) {
            return [["", 0]]
        }
        let histInput = []
        for (let i = 0; i < this.accessNetworkRichest.length; i++) {
            histInput.push(
                [this.accessNetworkRichest[i].nodeID, this.accessNetworkRichest[i].mana]
            )
        }
        return histInput
    }

    @computed
    get consensusHistogramInput() {
        if (this.consensusNetworkRichest === undefined || this.consensusNetworkRichest === null) {
            return [["", 0]]
        }
        let histInput = []
        for (let i = 0; i < this.consensusNetworkRichest.length; i++) {
            histInput.push(
                [this.consensusNetworkRichest[i].nodeID, this.consensusNetworkRichest[i].mana]
            )
        }
        return histInput
    }

    @computed
    get accessPercentile() {
        let per = 0.0;
        // find id
        if (this.accessNetworkRichest !== undefined && this.accessNetworkRichest !== null) {
            const isOwnID = (element) => element.nodeID === this.ownID;
            let index = this.accessNetworkRichest.findIndex(isOwnID);
            switch (index) {
                case -1:
                    break;
                default:
                    per = ((this.accessNetworkRichest.length - (index + 1)) / this.accessNetworkRichest.length) * 100;
                    break;
            }
        }
        return per
    }

    @computed
    get consensusPercentile() {
        let per = 0.0;
        // find id
        if ( this.consensusNetworkRichest !== undefined && this.consensusNetworkRichest !== null) {
            const isOwnID = (element) => element.nodeID === this.ownID;
            let index = this.consensusNetworkRichest.findIndex(isOwnID);
            switch (index) {
                case -1:
                    break;
                default:
                    per = ((this.consensusNetworkRichest.length - (index +1)) / this.consensusNetworkRichest.length) * 100;
            }
        }
        return per
    }
}

export default ManaStore;