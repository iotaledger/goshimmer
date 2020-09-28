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
    nodes: Array<Node>;
}

const maxRegistrySize = 100;

export class ManaStore {
    // mana values
    @observable manaValues: Array<any> = [];
    // list of 100 richest access mana nodes in their network, sorted in descending order
    @observable accessNetworkRichest: Array<Node> = [];
    // list of richest online access mana nodes in their network, sorted in descending order
    @observable accessOnlineRichest: Array<Node> = [];
    // list of 100 richest consensus mana nodes in their network, sorted in descending order
    @observable consensusNetworkRichest: Array<Node> = [];
    // list of richest online consensus mana nodes in their network, sorted in descending order
    @observable consensusOnlineRichest: Array<Node> = [];

    ownID: string;

    constructor() {
        this.manaValues = [];
        registerHandler(WSMsgType.Mana, this.addNewManaValue);
        registerHandler(WSMsgType.ManaMapOverall, this.updateNetworkRichest);
        registerHandler(WSMsgType.ManaMapOnline, this.updateOnlineRichest);
    };

    @action
    addNewManaValue = (manaMsg: ManaMsg) =>  {
        this.ownID = this.ownID? this.ownID : manaMsg.nodeID;
        if (this.manaValues.length === maxRegistrySize) {
            // shift if we already have enough values
            this.manaValues.shift();
        }
        let newManaData = [new Date(manaMsg.time*1000), manaMsg.access, manaMsg.consensus];
        this.manaValues.push(newManaData);
    }

    @action
    updateNetworkRichest = (msg: NetworkManaMsg) => {
        let tmp = msg;
        console.log(tmp);
        switch (msg.manaType) {
            case "Access Mana":
                this.accessNetworkRichest = msg.nodes;
                break;
            case "Consensus Mana":
                this.consensusNetworkRichest = msg.nodes;
                break;

        }
    }

    @action
    updateOnlineRichest = (msg: NetworkManaMsg) => {
        switch (msg.manaType) {
            case "Access Mana":
                this.accessOnlineRichest = msg.nodes;
                break;
            case "Consensus Mana":
                this.consensusOnlineRichest = msg.nodes;
                break;

        }
    }

    @computed
    get networkRichestFeedAccess() {
        if (this.accessNetworkRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.accessNetworkRichest.length; i++) {
            feed.push(
                <tr key={this.accessNetworkRichest[i].nodeID} style={{color: this.accessNetworkRichest[i].nodeID === this.ownID? 'red': 'black'}
                }>
                    <td> {i + 1} </td>
                    <td>{this.accessNetworkRichest[i].nodeID}</td>
                    <td>{this.accessNetworkRichest[i].mana.toFixed(2)}</td>
                </tr>
            )
        }
        return feed
    }

    @computed
    get networkRichestFeedConsensus() {
        if (this.consensusNetworkRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.consensusNetworkRichest.length; i++) {
            feed.push(
                <tr key={this.consensusNetworkRichest[i].nodeID}>
                    <td> {i + 1} </td>
                    <td>{this.consensusNetworkRichest[i].nodeID}</td>
                    <td>{this.consensusNetworkRichest[i].mana.toFixed(2)}</td>
                </tr>
            )
        }
        return feed
    }

    @computed
    get onlineRichestFeedAccess() {
        if (this.accessOnlineRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.accessOnlineRichest.length; i++) {
            feed.push(
                <tr key={this.accessOnlineRichest[i].nodeID}>
                    <td> {i + 1} </td>
                    <td>{this.accessOnlineRichest[i].nodeID}</td>
                    <td>{this.accessOnlineRichest[i].mana.toFixed(2)}</td>
                </tr>
            )
        }
        return feed
    }

    @computed
    get onlineRichestFeedConsensus() {
        if (this.consensusOnlineRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.consensusOnlineRichest.length; i++) {
            feed.push(
                <tr key={this.consensusOnlineRichest[i].nodeID}>
                    <td> {i + 1} </td>
                    <td>{this.consensusOnlineRichest[i].nodeID}</td>
                    <td>{this.consensusOnlineRichest[i].mana.toFixed(2)}</td>
                </tr>
            )
        }
        return feed
    }

    @computed
    get accessHistogramInput() {
        if (this.accessNetworkRichest === null || undefined) {
            return []
        }
        let histInput = []
        for (let i = 0; i < this.accessNetworkRichest.length; i++) {
            histInput.push(
                [this.accessNetworkRichest[i].nodeID, this.accessNetworkRichest[i].mana]
            )
        }
        return histInput
    }
}

export default ManaStore;