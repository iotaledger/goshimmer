export class utxoVertex {
    blkID: string;
    ID: string;
    inputs: Array<input>;
    outputs: Array<string>;
    conflictID: string;
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
    conflictID: string;
}

export class utxoConfirmationStateChanged {
    ID: string;
    confirmationState: string;
    isConfirmed: boolean;
    confirmationStateTime: number;
}
