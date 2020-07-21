import { action, computed, observable } from "mobx";
import { IStoredConflict } from "../models/IStoredConflict";
import { IFPCMessage } from "../models/messages/IFPCMessage";
import { Opinion } from "../models/opinion";
import { WSMsgType } from "../models/ws/wsMsgType";
import { registerHandler, unregisterHandler } from "../services/WS";

export class FPCStore {
    @observable
    private conflicts: IStoredConflict[] = [];

    @observable
    private currentConflict: string = "";

    private timerId: number;

    @computed
    public get nodeConflictGrid(): { [id: string]: number[] } | undefined {
        if (!this.currentConflict) {
            return undefined;
        }
        const currentConflict = this.conflicts.find(c => c.conflictID === this.currentConflict);
        if (!currentConflict) {
            return undefined;
        }
        return currentConflict.nodeOpinions;
    }

    @computed
    public get conflictGrid(): IStoredConflict[] {
        return this.conflicts.filter(c => c.likes !== undefined);
    }

    @action
    public updateCurrentConflict(id: string): void {
        this.currentConflict = id;
    }

    @action
    private addLiveFeed(msg: IFPCMessage): void {
        for (const conflictID in msg.conflictset) {
            for (const nodeID in msg.conflictset[conflictID].nodesview) {
                const voteContext = msg.conflictset[conflictID].nodesview[nodeID];
                
                let conflict = this.conflicts.find(c => c.conflictID === conflictID);
                if (!conflict) {
                    conflict = {
                        conflictID,
                        lastUpdated: Date.now(),
                        nodeOpinions: {}
                    };
                    this.conflicts.push(conflict);
                } else {
                    conflict.lastUpdated = Date.now();
                }

                if (!(nodeID in conflict.nodeOpinions)) {
                    conflict.nodeOpinions[nodeID] = [];
                } 
                conflict.nodeOpinions[nodeID] = voteContext.opinions;

                this.updateConflictState(conflict);
            }
        }
    }

    @action
    private updateConflictStates(): void {
        for (const conflict of this.conflicts) {
            this.updateConflictState(conflict);
        }

        const resolvedConflictIds = this.conflicts.filter(c =>
            Date.now() - c.lastUpdated > 10000 &&
            (c.likes === 0 || c.likes === Object.keys(c.nodeOpinions).length)
        ).map(c => c.conflictID);

        for (const conflictID of resolvedConflictIds) {
            this.conflicts = this.conflicts.filter(c => c.conflictID !== conflictID);
        }
    }

    @action
    private updateConflictState(conflict: IStoredConflict): void {
        let likes = 0;
        for (const nodeConflict in conflict.nodeOpinions) {
            if (conflict.nodeOpinions[nodeConflict].length > 0 && 
                conflict.nodeOpinions[nodeConflict][conflict.nodeOpinions[nodeConflict].length - 1] === Opinion.like) {
                likes++;
            }
        }
        conflict.likes = likes;
    }

    public start(): void {
        registerHandler(WSMsgType.fpc, (msg) => this.addLiveFeed(msg));
        this.timerId = setInterval(() => this.updateConflictStates(), 2000);
    }

    public stop(): void {
        unregisterHandler(WSMsgType.fpc);
        clearInterval(this.timerId);
    }
}

export default FPCStore;

