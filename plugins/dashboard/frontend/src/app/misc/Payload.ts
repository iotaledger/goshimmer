export enum PayloadType {
    Data = 0,
    Value = 1,
    Faucet = 2,
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
