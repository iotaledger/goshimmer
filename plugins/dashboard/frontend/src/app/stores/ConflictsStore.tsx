import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';

export class ConflictMessage {
    conflictID: string;
    arrivalTime: string;
    resolved: boolean;
    timeToResolve: number;
}

export class BranchMessage {
    branchID: string;
    conflictIDs: Array<string>;
    aw: number;
    gof: number;
    issuingTime: string;
    issuerNodeID: string;
}

// const liveFeedSize = 10;

export class ConflictsStore {
    // live feed
    @observable conflicts: Map<String, ConflictMessage>;
    @observable branches: Map<String, BranchMessage>;
    
    routerStore: RouterStore;
    nodeStore: NodeStore;

    constructor(routerStore: RouterStore, nodeStore: NodeStore) {
        this.routerStore = routerStore;
        this.nodeStore = nodeStore;
        this.conflicts = new Map;
        this.branches = new Map;
        registerHandler(WSMsgType.Conflict, this.updateConflicts);
        registerHandler(WSMsgType.Branch, this.updateBranches);
    }

    @action
    updateConflicts = (msg: ConflictMessage) => {
        this.conflicts.set(msg.conflictID, msg);
    };

    @action
    updateBranches = (msg: BranchMessage) => {
        this.branches.set(msg.branchID, msg);
    };
   
    @computed
    get conflictsLiveFeed() {
        let feed = [];
        for (let [key, conflict] of this.conflicts) {
            console.log(key, conflict);
            feed.push(
                        <tr key={conflict.conflictID}>
                            <td>
                                <Link to={`/explorer/output/${conflict.conflictID}`}>
                                {conflict.conflictID}
                                </Link>
                            </td>
                            <td>
                                {conflict.arrivalTime}
                            </td>
                            <td>
                                {conflict.resolved ? 'Yes' : 'No'}
                            </td>
                            <td>
                                {conflict.timeToResolve/1000000}
                            </td>
                        </tr>
                    );
            for (let [branchID, branch] of this.branches) {
                for(let conflictID of branch.conflictIDs){
                    if (conflictID === key) {
                        console.log("match:", branchID);
                    }
                }
            }
        } 
        return feed;
    }

}

export default ConflictsStore;