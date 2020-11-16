import {action, computed, observable} from 'mobx';
import * as React from "react";
import {Col, ListGroupItem, OverlayTrigger, Popover, Row} from "react-bootstrap";
import {WSMsgTypeDashboard} from "../models/ws/WSMsgTypeDashboard";
import {IManaMessage} from "../models/mana/IManaMessage";
import {INetworkManaMessage} from "../models/mana/INetworkManaMessage";
import {INode} from "../models/mana/INode";
import {IPledgeMessage} from "../models/mana/IPledgeMessage";
import {IRevokeMessage} from "../models/mana/IRevokeMessage";
import {displayManaUnit} from "../../../../../../dashboard/frontend/src/app/utils";
import Plus from "../../../../../../../plugins/dashboard/frontend/src/assets/plus.svg"
import Minus from "../../../../../../../plugins/dashboard/frontend/src/assets/minus.svg"
import {connectDashboardWebSocket, registerHandler} from "../services/WSmana";
import {autopeeringStore} from "../../main";

class ManaEvent {
    nodeID: string;
    time: Date;
    txID: string;

    constructor(nodeID: string, time: Date, txID: string) {
        this.nodeID = nodeID;
        this.time = time;
        this.txID = txID;
    }
}

class PledgeEvent extends ManaEvent{
    bm1: number;
    bm2: number;

    constructor(nodeID: string, time: Date, txID: string, bm1: number, bm2: number) {
        super(nodeID, time, txID);
        this.bm1 = bm1;
        this.bm2 = bm2;
    }
}

class RevokeEvent extends ManaEvent{
    bm1: number;

    constructor(nodeID: string, time: Date,  txID: string, bm1: number) {
        super(nodeID, time, txID);
        this.bm1 = bm1;
    }
}

const emptyRow = (<tr><td colSpan={4}>There are no nodes to view with the current search parameters.</td></tr>)
const emptyListItem = (<ListGroupItem>There are no events to view with the current search parameters.</ListGroupItem>)

// every 10 seconds, a new value arrives, so this is roughly 166 mins
const maxStoredManaValues = 1000;
// number of previous pledge/revoke events we keep track of. (/2 of plugins/dashboard/maxManaEventsBufferSize)
const maxEventsStored = 1000;

export class ManaStore {
    // mana values
    @observable manaValues: Array<any> = [];
    // first is accessm second consensus
    @observable prevManaValues: Array<number> = [0,0];
    // list of richest access mana nodes in  network, sorted in descending order
    @observable accessNetworkRichest: Array<INode> = [];
    @observable totalAccessNetwork: number = 0.0;
    // list of richest active access mana nodes in the network, sorted in descending order
    @observable accessActiveRichest: Array<INode> = [];
    @observable totalAccessActive: number = 0.0;
    // list of richest consensus mana nodes in their network, sorted in descending order
    @observable consensusNetworkRichest: Array<INode> = [];
    @observable totalConsensusNetwork: number = 0.0;
    // list of richest active consensus mana nodes in their network, sorted in descending order
    @observable consensusActiveRichest: Array<INode> = [];
    @observable totalConsensusActive: number = 0.0;

    @observable public searchNode = "";
    @observable public searchTxID = "";

    @observable accessEvents: Array<ManaEvent> = [];

    @observable consensusEvents: Array<ManaEvent> = [];
    @observable public dashboardWebsocketConnected: boolean = false;
    @observable public manaDashboardAddress: string

    ownID: string;

    constructor() {
        this.manaValues = [];
        registerHandler(WSMsgTypeDashboard.ManaMapOverall, this.updateNetworkRichest);
        registerHandler(WSMsgTypeDashboard.ManaMapOnline, this.updateActiveRichest);
        registerHandler(WSMsgTypeDashboard.ManaPledge, this.addNewPledge);
        registerHandler(WSMsgTypeDashboard.ManaRevoke, this.addNewRevoke);
    };

    @action
    updateNodeSearch(searchNode: string): void {
        this.searchNode = searchNode.trim();
    }

    @action
    updateTxSearch(searchTxID: string): void {
        this.searchTxID = searchTxID.trim();
    }

    @action
    addNewManaValue = (manaMsg: IManaMessage) =>  {
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
    updateNetworkRichest = (msg: INetworkManaMessage) => {
        switch (msg.manaType) {
            case "Access":
                this.totalAccessNetwork = msg.totalMana;
                this.accessNetworkRichest = msg.nodes;
                break;
            case "Consensus":
                this.totalConsensusNetwork = msg.totalMana;
                this.consensusNetworkRichest = msg.nodes;
                break;
        }
        //TODO: show access or consensus mana
        this.accessActiveRichest.forEach(node => {
            let per = this.accessPercentage(node)
            autopeeringStore.updateSizeBasedOnMana(node.shortNodeID, per)
        })
        this.addNewManaValue({
            nodeID: "",
            access: this.totalAccessNetwork,
            consensus: this.totalConsensusNetwork,
            time: new Date().getTime() * 1000
        })
    }

    @action
    updateActiveRichest = (msg: INetworkManaMessage) => {
        switch (msg.manaType) {
            case "Access":
                this.totalAccessActive = msg.totalMana;
                this.accessActiveRichest = msg.nodes;
                break;
            case "Consensus":
                this.totalConsensusActive = msg.totalMana;
                this.consensusActiveRichest = msg.nodes;
                break;
        }
    };

    @action
    addNewPledge = (msg: IPledgeMessage) => {
        switch (msg.manaType) {
            case "Access":
                this.handleNewPledgeEvent(this.accessEvents, msg);
                break;
            case "Consensus":
                this.handleNewPledgeEvent(this.consensusEvents, msg);
                break;
        }
    }


    @action
    public updateDashboardWebsocketConnect(connected: boolean): void {
        this.dashboardWebsocketConnected = connected
    }

    reconnect() {
        this.updateDashboardWebsocketConnect(false)
        setTimeout(() => {
            this.connect();
        }, 5000);
    }

    public connect(): void {
        connectDashboardWebSocket(this.manaDashboardAddress,
            () => this.updateDashboardWebsocketConnect(true),
            () => this.reconnect(),
            () => this.updateDashboardWebsocketConnect(false));
    }

    @action
    public setManaDashboardAddress(address: string): void {
        this.manaDashboardAddress = address
        this.connect()
    }

    handleNewPledgeEvent = (store: Array<ManaEvent>, msg: IPledgeMessage) => {
        if (store.length === maxEventsStored) {
            store.shift()
        }
        let newData = new PledgeEvent(
            msg.nodeID,
            new Date(msg.time*1000),
            msg.txID,
            msg.bm1,
            msg.bm2
        )
        store.push(newData)
    }

    @action
    addNewRevoke = (msg: IRevokeMessage) => {
        switch (msg.manaType) {
            case "Access":
                this.handleNewRevokeEvent(this.accessEvents, msg);
                break;
            case "Consensus":
                this.handleNewRevokeEvent(this.consensusEvents, msg);
                break;
        }
    }

    handleNewRevokeEvent = (store: Array<ManaEvent>, msg: IRevokeMessage) => {
        if (store.length === maxEventsStored) {
            store.shift()
        }
        let newData = new RevokeEvent(
            msg.nodeID,
            new Date(msg.time*1000),
            msg.txID,
            msg.bm1
        )
        store.push(newData)
    }

    nodeList = (leaderBoard: Array<INode>, manaSum: number) => {
        if (leaderBoard === null || undefined) {
            return []
        }
        let feed = new Array()
        let pushToFeed = (node: INode, i: number) => {
            feed.push(
                <tr
                    key={node.shortNodeID}
                    style={{
                        backgroundColor: node.shortNodeID === this.ownID ? '#e8ffff': 'white',
                    }}
                >
                    <td style={
                        {
                            borderTopLeftRadius: node.shortNodeID === this.ownID ? '10px': '0',
                            borderBottomLeftRadius: node.shortNodeID === this.ownID ? '10px': '0',
                        }
                    }> {i + 1} </td>
                    <td>{node.shortNodeID}</td>
                    <td>{displayManaUnit(node.mana)}</td>
                    <td style={
                        {
                            borderTopRightRadius: node.shortNodeID === this.ownID ? '10px': '0',
                            borderBottomRightRadius: node.shortNodeID === this.ownID ? '10px': '0',
                        }
                    }>{((node.mana / manaSum)*100.0).toFixed(2)}%</td>
                </tr>
            );
        };
        let callback = (node: INode, i: number) => {
            if (this.passesNodeFilter(node.shortNodeID)){
                pushToFeed(node, i);
            }
        };
        leaderBoard.forEach(callback);
        return feed
    }

    @computed
    get networkRichestFeedAccess() {
        let result =  this.nodeList(this.accessNetworkRichest, this.totalAccessNetwork);
        if (result.length === 0) {
            return [emptyRow];
        } else {
            return result;
        }
    }

    @computed
    get networkRichestFeedConsensus() {
        let result = this.nodeList(this.consensusNetworkRichest, this.totalConsensusNetwork);
        if (result.length === 0) {
            return [emptyRow];
        } else {
            return result;
        }
    }

    @computed
    get activeRichestFeedAccess() {
        let result = this.nodeList(this.accessActiveRichest, this.totalAccessActive);
        if (result.length === 0) {
            return [emptyRow];
        } else {
            return result;
        }
    }

    @computed
    get activeRichestFeedConsensus() {
        let result = this.nodeList(this.consensusActiveRichest, this.totalConsensusActive);
        if (result.length === 0) {
            return [emptyRow];
        } else {
            return result;
        }
    }

    @computed
    get accessHistogramInput() {
        if (this.accessNetworkRichest === undefined || this.accessNetworkRichest === null) {
            return [["", 0]]
        }
        let histInput = new Array()
        for (let i = 0; i < this.accessNetworkRichest.length; i++) {
            histInput.push(
                [this.accessNetworkRichest[i].shortNodeID, this.accessNetworkRichest[i].mana]
            )
        }
        return histInput
    }

    @computed
    get consensusHistogramInput() {
        if (this.consensusNetworkRichest === undefined || this.consensusNetworkRichest === null) {
            return [["", 0]]
        }
        let histInput = new Array()
        for (let i = 0; i < this.consensusNetworkRichest.length; i++) {
            histInput.push(
                [this.consensusNetworkRichest[i].shortNodeID, this.consensusNetworkRichest[i].mana]
            )
        }
        return histInput
    }

    private accessPercentage(ownNode: INode): number{
        return (ownNode.mana/this.totalAccessNetwork) * 100;
    }

    @computed
    get consensusPercentile() {
        let per = 0.0;
        // find id
        if ( this.consensusNetworkRichest !== undefined && this.consensusNetworkRichest !== null) {
            const isOwnID = (element) => element.shortNodeID === this.ownID;
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

    computeEventList = (evArr: Array<ManaEvent>) => {
        let result = new Array();
        result.push(
            <ListGroupItem
                style={{textAlign: 'center'}}
                key={'header'}
            >
                <Row>
                    <Col xs={1} className="m-auto">
                    </Col>
                    <Col>
                        NodeID
                    </Col>
                    <Col>
                        Tx ID
                    </Col>
                    <Col xs={5}>
                        Time
                    </Col>
                </Row>
            </ListGroupItem>
        )
        if (evArr === undefined || evArr === null) {
            return result
        }
        let pushToEventFeed = (element: ManaEvent, index) => {
            if (element instanceof PledgeEvent) {
                let popover = (ev: PledgeEvent) => {
                    return (
                        <Popover id={ev.nodeID + index.toString()}>
                            <Popover.Title as="h3">Mana Pledged</Popover.Title>
                            <Popover.Content>
                                <div>Base Mana 1: <strong>+{displayManaUnit(ev.bm1)}</strong></div>
                                <div>Base Mana 2: <strong>+{displayManaUnit(ev.bm2)}</strong></div>
                                <div>With Transaction: <strong><a onClick={() => navigator.clipboard.writeText(ev.txID)}>{ev.txID}</a></strong></div>
                                <div>To NodeID:  <strong>{ev.nodeID}</strong></div>
                                <div>Time of Pledge:  <strong>{ev.time.toLocaleTimeString()}</strong></div>
                            </Popover.Content>
                        </Popover>
                    )
                }
                result.push(
                    <OverlayTrigger key={element.nodeID + index.toString()} trigger="focus" placement="top" overlay={popover(element)}>
                        <ListGroupItem
                            style={{backgroundColor: '#41aea9', color: 'white', textAlign: 'center'}}
                            key={element.nodeID + index.toString(10)}
                            //onClick={() => do something on click}>
                            as={'button'}
                        >
                            <Row>
                                <Col xs={1} className="m-auto">
                                    <img src={Plus} alt="Plus" width={'20px'} className="d-block mx-auto"/>
                                </Col>
                                <Col>
                                    {element.nodeID}
                                </Col>
                                <Col>
                                    {element.txID.substring(0, 7) + '...'}
                                </Col>
                                <Col xs={5}>
                                    {element.time.toLocaleString()}
                                </Col>
                            </Row>
                        </ListGroupItem>
                    </OverlayTrigger>
                )
            } else if (element instanceof RevokeEvent){
                let popover = (ev: RevokeEvent) => {
                    return (
                        <Popover id={ev.nodeID + index.toString()}>
                            <Popover.Title as="h3">Mana Revoked</Popover.Title>
                            <Popover.Content>
                                <div>Base Mana 1: <strong>-{displayManaUnit(ev.bm1)}</strong></div>
                                <div>With Transaction: <strong><a onClick={() => navigator.clipboard.writeText(ev.txID)}>{ev.txID}</a></strong></div>
                                <div>From NodeID:  <strong>{ev.nodeID}</strong></div>
                                <div>Time of Revoke:  <strong>{ev.time.toLocaleTimeString()}</strong></div>
                            </Popover.Content>
                        </Popover>
                    )
                }
                // it's a revoke event then
                result.push(
                    <OverlayTrigger key={element.nodeID + index.toString()} trigger="focus" placement="top" overlay={popover(element)}>
                        <ListGroupItem
                            style={{backgroundColor: '#213e3b', color: 'white', textAlign: 'center'}}
                            key={element.nodeID + index.toString(10)}
                            //onClick={() => do something on click}>
                            as={'button'}
                        >
                            <Row>
                                <Col xs={1}>
                                    <img src={Minus} alt="Minus" width={'20px'} className=""/>
                                </Col>
                                <Col>
                                    {element.nodeID}
                                </Col>
                                <Col>
                                    {element.txID.substring(0, 7) + '...'}
                                </Col>
                                <Col xs={5}>
                                    {element.time.toLocaleString()}
                                </Col>
                            </Row>
                        </ListGroupItem>
                    </OverlayTrigger>
                )
            }
        };
        // && this.passesTimeFilter(event.time) {
        let callback = (event: ManaEvent, i: number) => {
            if (this.passesNodeFilter(event.nodeID) && this.passesTxFilter(event.txID)){
                pushToEventFeed(event, i);
            }
        };
        // reverse traverse bc oldest event is the first
        evArr.reverse().forEach(callback)
        return result;
    }

    @computed
    get accessEventList() {
        let result = this.computeEventList(this.accessEvents);
        if (result.length === 1) {
            result.push(emptyListItem);
        }
        return result;
    }

    @computed
    get consensusEventList() {
        let result = this.computeEventList(this.consensusEvents);
        if (result.length === 1) {
            result.push(emptyListItem);
        }
        return result;
    }

    passesNodeFilter = (nodeID: string) : boolean => {
        if (this.searchNode.trim().length === 0) {
            // node filter is disabled, anything passes the filter
            return true;
        } else if (nodeID.toLowerCase().includes(this.searchNode.toLowerCase())){
            // node filter is enabled, nodeID contains search term
            return true;
        }
        // filter enabled but nodeID doesn't pass
        return false;
    }

    passesTxFilter = (txID: string) : boolean => {
        if (this.searchTxID.trim().length === 0) {
            // txID filter is disabled, anything passes the filter
            return true;
        } else if (txID.toLowerCase().includes(this.searchTxID.toLowerCase())){
            // txID filter is enabled, txID contains search term
            return true;
        }
        // filter enabled but txID doesn't pass
        return false;
    }
}

export default ManaStore;