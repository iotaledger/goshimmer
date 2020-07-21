import FPCStore from "../../stores/FPCStore";

export interface FPCItemProps {
    fpcStore?: FPCStore;
    conflictID: string;
    likes?: number;
    nodeOpinions: { [id: string]: number[] };
}
