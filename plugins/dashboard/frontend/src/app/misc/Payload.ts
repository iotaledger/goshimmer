export enum PayloadType {
    Data = 0,
    Value = 1,
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

// Value payload
export class ValuePayload {
    payload_id: string;
    parent1_id: string;
    parent2_id: string;
    tx_id: string;
    inputs: Array<Inputs>;
    outputs: Array<Outputs>;
    data: string;
}

export class Inputs {
    address: string;
}

export class Outputs {
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
        case PayloadType.Value:
            return "Value"
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