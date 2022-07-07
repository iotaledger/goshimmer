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

export class ConflictBlock {
    conflictID: string;
    conflictIDs: Array<string>;
    confirmationState: number;
    issuingTime: number;
    issuerNodeID: string;
}

// const liveFeedSize = 10;

export class ConflictsStore {
    // live feed
    @observable conflicts: Map<String, ConflictBlock>;
    @observable conflicts: Map<String, ConflictBlock>;
    
    routerStore: RouterStore;
    nodeStore: NodeStore;

    constructor(routerStore: RouterStore, nodeStore: NodeStore) {
        this.routerStore = routerStore;
        this.nodeStore = nodeStore;
        this.conflicts = new Map;
        this.conflicts = new Map;
        registerHandler(WSBlkType.Conflict, this.updateConflicts);
        registerHandler(WSBlkType.Conflict, this.updateConflicts);
    }

    @action
    updateConflicts = (blk: ConflictBlock) => {
        this.conflicts.set(blk.conflictID, blk);
    };

    @action
    updateConflicts = (blk: ConflictBlock) => {
        this.conflicts.set(blk.conflictID, blk);
    };
   
    @computed
    get conflictsLiveFeed() {
        // sort conflicts by time and ID to prevent "jumping"
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

            // only render and show conflicts if it has been clicked
            if (!conflict.shown) {
                continue
            }

            // sort conflicts by time and ID to prevent "jumping"
            let conflictsArr = Array.from(this.conflicts.values());
            conflictsArr.sort((x: ConflictBlock, y: ConflictBlock): number => {
                   return x.issuingTime - y.issuingTime || x.conflictID.localeCompare(y.conflictID)
                }
            )

            let conflicts = [];
            for (let conflict of conflictsArr) {
                for(let conflictID of conflict.conflictIDs){
                    if (conflictID === conflict.conflictID) {
                        conflicts.push(
                                    <tr key={conflict.conflictID} className={conflict.confirmationState > ConfirmationState.Accepted ? "table-success" : ""}>
                                        <td>
                                            <Link to={`/explorer/conflict/${conflict.conflictID}`}>
                                                {conflict.conflictID}
                                            </Link>
                                        </td>
                                        <td>{resolveConfirmationState(conflict.confirmationState)}</td>
                                        <td> {new Date(conflict.issuingTime * 1000).toLocaleString()}</td>
                                        <td>{conflict.issuerNodeID}</td>
                                    </tr>
                        );
                    }
                }
            }
            feed.push(
                <tr key={conflict.conflictID+"_conflicts"}>
                    <td colSpan={4}>
                        <Table size="sm">
                            <thead>
                            <tr>
                                <th>ConflictID</th>
                                <th>ConfirmationState</th>
                                <th>IssuingTime</th>
                                <th>Issuer NodeID</th>
                            </tr>
                            </thead>
                            <tbody>
                            {conflicts}
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
