import {WSBlkType} from "./wsBlkType";

export interface WSBlock {
    type: WSBlkType;
    data: unknown;
}
