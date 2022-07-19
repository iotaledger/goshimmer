import {action, computed, observable} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';
import {Table} from "react-bootstrap";
import {ConfirmationState, resolveConfirmationState} from "app/utils/confirmation_state";

export class ConflictSet {
    conflictSetID: string;
    arrivalTime: number;
    resolved: boolean;
    timeToResolve: number;
    shown: boolean;
}

export class Conflict {
    conflictID: string;
    conflictSetIDs: Array<string>;
    confirmationState: number;
    issuingTime: number;
    issuerNodeID: string;
}

// const liveFeedSize = 10;

export class ConflictsStore {
    // live feed
    @observable conflictSets: Map<String, ConflictSet>;
    @observable conflicts: Map<String, Conflict>;
    
    routerStore: RouterStore;
    nodeStore: NodeStore;

    constructor(routerStore: RouterStore, nodeStore: NodeStore) {
        this.routerStore = routerStore;
        this.nodeStore = nodeStore;
        this.conflictSets = new Map;
        this.conflicts = new Map;
        registerHandler(WSMsgType.ConflictSet, this.updateConflictSets);
        registerHandler(WSMsgType.Conflict, this.updateConflicts);
    }

    @action
    updateConflictSets = (blk: ConflictSet) => {
        this.conflictSets.set(blk.conflictSetID, blk);
    };

    @action
    updateConflicts = (blk: Conflict) => {
        this.conflicts.set(blk.conflictID, blk);
    };
   
    @computed
    get conflictsLiveFeed() {
        // sort branches by time and ID to prevent "jumping"
        let conflictsArr = Array.from(this.conflictSets.values());
        conflictsArr.sort((x: ConflictSet, y: ConflictSet): number => {
                return y.arrivalTime - x.arrivalTime || x.conflictSetID.localeCompare(y.conflictSetID);
            }
        )

        let feed = [];
        for (let conflict of conflictsArr) {
            feed.push(
                <tr key={conflict.conflictSetID} onClick={() => conflict.shown = !conflict.shown} style={{cursor:"pointer"}}>
                    <td>
                        <Link to={`/explorer/output/${conflict.conflictSetID}`}>
                            {conflict.conflictSetID}
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
            let branchesArr = Array.from(this.conflicts.values());
            branchesArr.sort((x: Conflict, y: Conflict): number => {
                   return x.issuingTime - y.issuingTime || x.conflictID.localeCompare(y.conflictID)
                }
            )

            let branches = [];
            for (let branch of branchesArr) {
                for(let conflictID of branch.conflictSetIDs){
                    if (conflictID === conflict.conflictSetID) {
                        branches.push(
                                    <tr key={branch.conflictID} className={branch.confirmationState > ConfirmationState.Accepted ? "table-success" : ""}>
                                        <td>
                                            <Link to={`/explorer/branch/${branch.conflictID}`}>
                                                {branch.conflictID}
                                            </Link>
                                        </td>
                                        <td>{resolveConfirmationState(branch.confirmationState)}</td>
                                        <td> {new Date(branch.issuingTime * 1000).toLocaleString()}</td>
                                        <td>{branch.issuerNodeID}</td>
                                    </tr>
                        );
                    }
                }
            }
            feed.push(
                <tr key={conflict.conflictSetID+"_branches"}>
                    <td colSpan={4}>
                        <Table size="sm">
                            <thead>
                            <tr>
                                <th>BranchID</th>
                                <th>ConfirmationState</th>
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

        return feed;
    }

}

export default ConflictsStore;
