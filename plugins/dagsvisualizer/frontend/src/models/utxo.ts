export class utxoVertex {
    msgID: string;
    ID: string;
    inputs: Array<input>;
    outputs: Array<string>;
    branchID: string;
    isConfirmed: boolean;
    gof: string;
    confirmedTime: number;
}

export class input {
    type: string;
    referencedOutputID: any;
}

export class utxoBooked {
    ID: string;
    branchID: string;
}

export class utxoConfirmed {
    ID: string;
    gof: string;
    confirmedTime: number;
}
