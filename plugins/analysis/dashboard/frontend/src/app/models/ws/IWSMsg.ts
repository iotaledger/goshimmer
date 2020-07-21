import { WSMsgType } from "./wsMsgType";

export interface WSMessage {
    type: WSMsgType;
    data: unknown;
}
