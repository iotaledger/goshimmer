import { IConflict } from "./IConflict";

export interface IFPCMessage {
    conflictset: { [id: string]: IConflict };
}
