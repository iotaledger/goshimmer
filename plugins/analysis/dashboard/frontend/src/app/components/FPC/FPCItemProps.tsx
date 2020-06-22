import FPCStore from "app/stores/FPCStore";

export interface FPCItemProps {
    fpcStore?: FPCStore;
    conflictID: string;
    likes?: number;
    nodeOpinions: { nodeID: string; opinion: number }[];
}
