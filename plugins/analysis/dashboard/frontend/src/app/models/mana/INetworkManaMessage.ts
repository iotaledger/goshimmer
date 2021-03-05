import {INode} from "./INode";

export interface INetworkManaMessage {
    manaType: string;
    totalMana: number;
    nodes: Array<INode>;
}