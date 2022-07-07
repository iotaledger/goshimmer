export class utxoVertex {
    blkID: string;
    ID: string;
    inputs: Array<input>;
    outputs: Array<string>;
    branchID: string;
    isConfirmed: boolean;
    confirmationState: string;
    confirmationStateTime: number;
}

export class input {
    type: string;
    referencedOutputID: any;
}

export class utxoBooked {
    ID: string;
    branchID: string;
}

export class utxoConfirmationStateChanged {
    ID: string;
    confirmationState: string;
    isConfirmed: boolean;
    confirmationStateTime: number;
}
