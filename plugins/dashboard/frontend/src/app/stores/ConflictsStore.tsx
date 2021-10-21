import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';
import {Table} from "react-bootstrap";
import {GoF} from "app/stores/ExplorerStore";

export class ConflictMessage {
    conflictID: string;
    arrivalTime: number;
    resolved: boolean;
    timeToResolve: number;
    shown: boolean;
    updatedAt: Date;
}

export class BranchMessage {
    branchID: string;
    conflictIDs: Array<string>;
    aw: number;
    gof: number;
    issuingTime: number;
    issuerNodeID: string;
    updatedAt: Date;
}

const maxStoredConflicts = 100;
const maxStoredBranches = 100;

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
        msg.updatedAt = new Date();
        this.conflicts.set(msg.conflictID, msg);
    };

    @action
    updateBranches = (msg: BranchMessage) => {
        msg.updatedAt = new Date();
        this.branches.set(msg.branchID, msg);
    };
   
    @computed
    get conflictsLiveFeed() {
        let feed = [];
        let oldestConflictUpdate: ConflictMessage;
        let oldestConflictUpdatedAt = new Date(0);
        let oldestBranchUpdate: BranchMessage;
        let oldestBranchUpdatedAt = new Date(0);
        for (let [key, conflict] of this.conflicts) {
            if(conflict.updatedAt < oldestConflictUpdatedAt) {
                oldestConflictUpdatedAt = conflict.updatedAt;
                oldestConflictUpdate = conflict;
            }
            feed.push(
                <tr key={conflict.conflictID} onClick={() => conflict.shown = !conflict.shown} style={{cursor:"pointer"}}>
                    <td>
                        <Link to={`/explorer/output/${conflict.conflictID}`}>
                            {conflict.conflictID}
                        </Link>
                    </td>
                    <td>
                        {new Date(conflict.arrivalTime * 1000).toLocaleString()}
                    </td>
                    <td>
                        {conflict.resolved ? 'Yes' : 'No'}
                    </td>
                    <td>
                        {conflict.timeToResolve/1000000}
                    </td>
                </tr>
            );

            // only render and show branches if it has been clicked
            if (!conflict.shown) {
                continue
            }

            // sort branches by time and ID to prevent "jumping"
            let branchesArr = Array.from(this.branches.values());
            branchesArr.sort((x: BranchMessage, y: BranchMessage): number => {
                   return x.issuingTime - y.issuingTime || x.branchID.localeCompare(y.branchID)
                }
            )

            let branches = [];
            for (let branch of branchesArr) {
                if(branch.updatedAt < oldestBranchUpdatedAt) {
                    oldestBranchUpdatedAt = branch.updatedAt;
                    oldestBranchUpdate = branch;
                }
                for(let conflictID of branch.conflictIDs){
                    if (conflictID === key) {
                        branches.push(
                                    <tr key={branch.branchID} className={branch.gof == GoF.High ? "table-success" : ""}>
                                        <td>
                                            <Link to={`/explorer/branch/${branch.branchID}`}>
                                                {branch.branchID}
                                            </Link>
                                        </td>
                                        <td>{branch.aw}</td>
                                        <td>{branch.gof}</td>
                                        <td> {new Date(branch.issuingTime * 1000).toLocaleString()}</td>
                                        <td>{branch.issuerNodeID}</td>
                                    </tr>
                        );
                    }
                }
            }
            feed.push(
                <tr key={conflict.conflictID+"_branches"}>
                    <td colSpan={4}>
                        <Table size="sm">
                            <thead>
                            <tr>
                                <th>BranchID</th>
                                <th>AW</th>
                                <th>GoF</th>
                                <th>IssuingTime</th>
                                <th>Issuer NodeID</th>
                            </tr>
                            </thead>
                            <tbody>
                            {branches}
                            </tbody>
                        </Table>
                    </td>
                </tr>
            );
        }

        if(this.conflicts.size === maxStoredConflicts) {
            this.conflicts.delete(oldestConflictUpdate.conflictID);
        }
        if(this.branches.size === maxStoredBranches){
            this.branches.delete(oldestBranchUpdate.branchID);
        }

        return feed;
    }

}

export default ConflictsStore;