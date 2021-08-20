import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';
import {Collapse, Table} from "react-bootstrap";
import {GoF} from "app/stores/ExplorerStore";

export class ConflictMessage {
    conflictID: string;
    arrivalTime: number;
    resolved: boolean;
    timeToResolve: number;
}

export class BranchMessage {
    branchID: string;
    conflictIDs: Array<string>;
    aw: number;
    gof: number;
    issuingTime: number;
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
                <tr key={conflict.conflictID} >
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

            // TODO: this might be inefficient with many branches. might be better to keep a list of branches with each conflict.
            //   would also be great to sort this deterministically.
            let branches = [];
            for (let [branchID, branch] of this.branches) {
                for(let conflictID of branch.conflictIDs){
                    if (conflictID === key) {
                        console.log("match:", branchID);
                        branches.push(
                                    <tr className={branch.gof == GoF.High ? "table-success" : ""}>
                                        <td>
                                            <Link to={`/explorer/branch/${branchID}`}>
                                                {branchID}
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
                <Collapse in={open}>
                <tr>
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
                </Collapse>
            );
        }

        return feed;
    }

}

export default ConflictsStore;