export enum PayloadType {
    Data = 0,
    Transaction = 1337,
    Faucet = 2,
    Statement = 3,
    Drng = 111,
    SyncBeacon = 200,
}

export enum DrngSubtype {
    Default = 0,
    Cb      = 1,
}

// BasicPayload
export class BasicPayload {
    content_title: string;
    content: string;
}

// DrngPayload
export class DrngPayload {
    subpayload_type: string;
    instance_id: number;
    drngpayload: any;
}

export class DrngCbPayload {
    round: number;
    prev_sig: string;
    sig: string;
    dpk: string;
}

// Transaction payload
export class TransactionPayload {
    tx_id: string;
    tx_essence: TransactionEssence;
    unlock_blocks: Array<string>;
}

export class TransactionEssence {
    version: number;
    timestamp: number;
    access_pledge_id: string;
    cons_pledge_id: string;
    inputs: Array<Input>;
    outputs: Array<Output>;
    data: string;
}

export class Input {
    output_id: string;
    address: string;
    balance: Array<Balance>;
}

export class Output {
    output_id: string;
    address: string;
    balance: Array<Balance>;
}

export class Balance {
    value: number;
    color: string;
}

// Sync beacon payload
export class SyncBeaconPayload {
    sent_time: number;
}

export class StatementPayload {
    conflicts: Array<Conflict>;
    timestamps: Array<Timestamp>;
}

export class Conflict {
    tx_id: string;
    opinion: Opinion;
}

export class Timestamp {
    msg_id: string;
    opinion: Opinion;
}

// @ts-ignore
export class Opinion {
    value: string;
    round: number;
}

export function getPayloadType(p: number){
    switch (p) {
        case PayloadType.Data:
            return "Data"
        case PayloadType.Transaction:
            return "Transaction"
        case PayloadType.Statement:
            return "Statement"
        case PayloadType.Drng:
            return "Drng"
        case PayloadType.Faucet:
            return "Faucet"
        case PayloadType.SyncBeacon:
            return "SyncBeacon"
        default:
            return "Unknown"
    }
}