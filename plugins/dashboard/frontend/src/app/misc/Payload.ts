export enum PayloadType {
    Data = 0,
    Transaction = 1,
    Faucet = 3,
}

// BasicPayload
export class BasicPayload {
    content_title: string;
    content: string;
}

// Transaction payload
export class TransactionPayload {
    txID: string;
    transaction: Transaction;
}

export class FaucetPayload {
    address: string;
    accessManaPledgeID: string;
    consensusManaPledgeID: string;
    nonce: number;
}

export class Transaction {
    version: number;
    timestamp: number;
    accessPledgeID: string;
    consensusPledgeID: string;
    inputs: Array<Input>;
    outputs: Array<Output>;
    unlockBlocks: Array<UnlockBlock>;
    dataPayload: any;
}

export class Input {
    type: string;
    referencedOutputID: OutputID;
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

export class UnlockBlock {
    type: string;
    referencedIndex: number;
    signatureType: number;
    publicKey: string;
    signature: string;
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
    isDelegated: boolean;
    delegationTimelock: number;
    governingAddress: string;

    stateData: any;
    governanceMetadata: any;
    immutableData: any;
}

export class ExtendedLockedOutput {
    balances: Map<string,number>;
    address: string
    fallbackAddress?: string;
    fallbackDeadline?: number;
    timelock?: number;
    payload: any;

}

export class Balance {
    value: number;
    color: string;
}

export class Conflict {
    tx_id: string;
    opinion: Opinion;
}

export class Timestamp {
    blk_id: string;
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
        case PayloadType.Faucet:
            return "Faucet"
        default:
            return "Unknown"
    }
}
