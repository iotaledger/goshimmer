import { registerHandler, WSMsgType, unregisterHandler } from "app/misc/WS";
import { action, computed, observable } from "mobx";

enum Opinion {
    Like = 1,
    Dislike
}

class VoteContext {
    nodeid: string;
    rounds: number;
    opinions: number[];
    status: number;
}

class Conflict {
    nodesview: Map<string, VoteContext>
}

export class FPCMessage {
    conflictset: Map<string, Conflict>
}

export class FPCStore {
    @observable msg: FPCMessage = null;

    @observable conflicts: {
        conflictID: string;
        nodeOpinions: { nodeID: string; opinion: number }[];
        lastUpdated: number;
        likes?: number;
    }[] = [];

    @observable currentConflict = "";

    timerId: NodeJS.Timer;

    constructor() {
    }

    start() {
        registerHandler(WSMsgType.FPC, this.addLiveFeed);
        this.timerId = setInterval(() => this.updateConflictStates(), 2000);
    }

    stop() {
        unregisterHandler(WSMsgType.FPC);
        clearInterval(this.timerId);
    }

    @action
    updateCurrentConflict = (id: string) => {
        this.currentConflict = id;
    }

    @action
    addLiveFeed = (msg: FPCMessage) => {
        let conflictIDs = Object.keys(msg.conflictset);
        if (!conflictIDs) return;

        for (const conflictID of conflictIDs) {
            let nodeIDs = Object.keys(msg.conflictset[conflictID].nodesview);
            for (const nodeID of nodeIDs) {
                let voteContext = msg.conflictset[conflictID].nodesview[nodeID];
                let latestOpinion = voteContext.opinions[voteContext.opinions.length - 1];

                let conflict = this.conflicts.find(c => c.conflictID === conflictID);
                if (!conflict) {
                    conflict = {
                        conflictID,
                        lastUpdated: Date.now(),
                        nodeOpinions: []
                    };
                    this.conflicts.push(conflict);
                } else {
                    conflict.lastUpdated = Date.now();
                }

                const nodeOpinionIndex = conflict.nodeOpinions.findIndex(no => no.nodeID === nodeID);
                if (nodeOpinionIndex >= 0) {
                    conflict.nodeOpinions[nodeOpinionIndex].opinion = latestOpinion;
                } else {
                    conflict.nodeOpinions.push({
                        nodeID,
                        opinion: latestOpinion
                    });
                }

                this.updateConflictState(conflict);
            }
        }
    }

    @action
    updateConflictStates() {
        for (let i = 0; i < this.conflicts.length; i++) {
            this.updateConflictState(this.conflicts[i]);
        }

        const resolvedConflictIds = this.conflicts.filter(c =>
            Date.now() - c.lastUpdated > 10000 &&
            (c.likes === 0 || c.likes === Object.keys(c.nodeOpinions).length)
        ).map(c => c.conflictID);

        for (const conflictID of resolvedConflictIds) {
            this.conflicts = this.conflicts.filter(c => c.conflictID !== conflictID)
        }
    }

    @action
    updateConflictState(conflict) {
        conflict.likes = conflict.nodeOpinions.filter((nodeOpinion) => nodeOpinion.opinion === Opinion.Like).length;
    }

    @computed
    get nodeConflictGrid() {
        if (!this.currentConflict) return;
        const currentConflict = this.conflicts.find(c => c.conflictID === this.currentConflict);
        if (!currentConflict) return;
        return currentConflict.nodeOpinions;
    }

    @computed
    get conflictGrid() {
        return this.conflicts.filter(c => c.likes !== undefined);
    }
}

export default FPCStore;

