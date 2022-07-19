import {INode} from "./INode";

export interface INetworkManaBlock {
    manaType: string;
    totalMana: number;
    nodes: Array<INode>;
}