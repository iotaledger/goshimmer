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
    type: string;
    referencedOutputID: string;
    output: Output;
}

export class Output {
    outputID: OutputID;
    type: string;
    output: any;
}

export class OutputID {
    base58: string;
    transactionID: string;
    outputIndex: number;
}

export class SigLockedSingleOutput {
    balance: number;
    address: string;
}

export class SigLockedColoredOutput {
    balances: Map<string,number>;
    address: string;
}

export class AliasOutput {
    balances: Map<string,number>;
    aliasAddress: string;
    stateAddress: string;
    stateIndex: number;
    isGovernanceUpdate: boolean;
    isOrigin: boolean;
    governingAddress: string;

    stateData: any;
    immutableData: any;
}

export class ExtendedLockedOutput {
    balances: Map<string,number>;
    address: string
    fallbackAddress: string;
    fallbackDeadline: number;
    timelock: number;
    payload: any;

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