import { IVoteContext } from "./IVoteContext";

export interface IConflict {
    nodesview: { [id: string]: IVoteContext };
    modified: number;
}
