import {action, computed, observable} from 'mobx';
import {registerHandler, WSBlkType} from "app/misc/WS";
import * as React from "react";
import {RouterStore,} from "mobx-react-router";
import {Link} from "react-router-dom";
import NodeStore from './NodeStore';
import {Table} from "react-bootstrap";
import {ConfirmationState, resolveConfirmationState} from "app/utils/confirmation_state";

export class ConflictBlock {
    conflictID: string;
    arrivalTime: number;
    resolved: boolean;
    timeToResolve: number;
    shown: boolean;
}

export class BranchBlock {
    branchID: string;
    conflictIDs: Array<string>;
    confirmationState: number;
    issuingTime: number;
    issuerNodeID: string;
}

// const liveFeedSize = 10;

export class ConflictsStore {
    // live feed
    @observable conflicts: Map<String, ConflictBlock>;
    @observable branches: Map<String, BranchBlock>;
    
    routerStore: RouterStore;
    nodeStore: NodeStore;

    constructor(routerStore: RouterStore, nodeStore: NodeStore) {
        this.routerStore = routerStore;
        this.nodeStore = nodeStore;
        this.conflicts = new Map;
        this.branches = new Map;
        registerHandler(WSBlkType.Conflict, this.updateConflicts);
        registerHandler(WSBlkType.Conflict, this.updateBranches);
    }

    @action
    updateConflicts = (blk: ConflictBlock) => {
        this.conflicts.set(blk.conflictID, blk);
    };

    @action
    updateBranches = (blk: BranchBlock) => {
        this.branches.set(blk.branchID, blk);
    };
   
    @computed
    get conflictsLiveFeed() {
        // sort branches by time and ID to prevent "jumping"
        let conflictsArr = Array.from(this.conflicts.values());
        conflictsArr.sort((x: ConflictBlock, y: ConflictBlock): number => {
                return y.arrivalTime - x.arrivalTime || x.conflictID.localeCompare(y.conflictID);
            }
        )

        let feed = [];
        for (let conflict of conflictsArr) {
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
            branchesArr.sort((x: BranchBlock, y: BranchBlock): number => {
                   return x.issuingTime - y.issuingTime || x.branchID.localeCompare(y.branchID)
                }
            )

            let branches = [];
            for (let branch of branchesArr) {
                for(let conflictID of branch.conflictIDs){
                    if (conflictID === conflict.conflictID) {
                        branches.push(
                                    <tr key={branch.branchID} className={branch.confirmationState > ConfirmationState.Accepted ? "table-success" : ""}>
                                        <td>
                                            <Link to={`/explorer/branch/${branch.branchID}`}>
                                                {branch.branchID}
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
                <tr key={conflict.conflictID+"_branches"}>
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
