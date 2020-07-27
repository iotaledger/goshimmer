export interface IStoredConflict {
    conflictID: string;
    nodeOpinions: { [id: string]: number[] };
    lastUpdated: number;
    likes?: number;
}