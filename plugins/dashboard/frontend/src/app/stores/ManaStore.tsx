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

const maxRegistrySize = 100;

export class ManaStore {
    // mana values
    @observable manaValues: Array<any> = [];
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

    ownID: string;

    constructor() {
        this.manaValues = [];
        registerHandler(WSMsgType.Mana, this.addNewManaValue);
        registerHandler(WSMsgType.ManaMapOverall, this.updateNetworkRichest);
        registerHandler(WSMsgType.ManaMapOnline, this.updateActiveRichest);
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
    }

    @computed
    get networkRichestFeedAccess() {
        if (this.accessNetworkRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.accessNetworkRichest.length; i++) {
            feed.push(
                <tr
                    key={this.accessNetworkRichest[i].nodeID}
                    style={{color: this.accessNetworkRichest[i].nodeID === this.ownID? 'red': 'black'}}
                >
                    <td> {i + 1} </td>
                    <td>{this.accessNetworkRichest[i].nodeID}</td>
                    <td>{this.accessNetworkRichest[i].mana.toFixed(2)}</td>
                    <td>{((this.accessNetworkRichest[i].mana / this.totalAccessNetwork)*100.0).toFixed(2)}%</td>
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
                <tr
                    key={this.consensusNetworkRichest[i].nodeID}
                    style={{color: this.consensusNetworkRichest[i].nodeID === this.ownID? 'red': 'black'}}
                >
                    <td> {i + 1} </td>
                    <td>{this.consensusNetworkRichest[i].nodeID}</td>
                    <td>{this.consensusNetworkRichest[i].mana.toFixed(2)}</td>
                    <td>{((this.consensusNetworkRichest[i].mana / this.totalConsensusNetwork)*100.0).toFixed(2)}%</td>
                </tr>
            )
        }
        return feed
    }

    @computed
    get activeRichestFeedAccess() {
        if (this.accessActiveRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.accessActiveRichest.length; i++) {
            feed.push(
                <tr
                    key={this.accessActiveRichest[i].nodeID}
                    style={{color: this.accessActiveRichest[i].nodeID === this.ownID? 'red': 'black'}}
                >
                    <td> {i + 1} </td>
                    <td>{this.accessActiveRichest[i].nodeID}</td>
                    <td>{this.accessActiveRichest[i].mana.toFixed(2)}</td>
                    <td>{((this.accessActiveRichest[i].mana / this.totalAccessActive)*100.0).toFixed(2)}%</td>
                </tr>
            )
        }
        return feed
    }

    @computed
    get activeRichestFeedConsensus() {
        if (this.consensusActiveRichest === null || undefined) {
            return []
        }
        let feed = []
        for (let i= 0; i< this.consensusActiveRichest.length; i++) {
            feed.push(
                <tr
                    key={this.consensusActiveRichest[i].nodeID}
                    style={{color: this.consensusActiveRichest[i].nodeID === this.ownID? 'red': 'black'}}
                >
                    <td> {i + 1} </td>
                    <td>{this.consensusActiveRichest[i].nodeID}</td>
                    <td>{this.consensusActiveRichest[i].mana.toFixed(2)}</td>
                    <td>{((this.consensusActiveRichest[i].mana / this.totalConsensusActive)*100.0).toFixed(2)}%</td>
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