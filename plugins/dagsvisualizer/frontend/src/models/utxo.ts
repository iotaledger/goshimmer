export class utxoVertex {
    msgID: string;
    ID: string;
    inputs: Array<input>;
    outputs: Array<string>;
    branchID: string;
    isConfirmed: boolean;
    gof: string;
    gofTime: number;
}

export class input {
    type: string;
    referencedOutputID: any;
}

export class utxoBooked {
    ID: string;
    branchID: string;
}

export class utxoGoFChanged {
    ID: string;
    gof: string;
    isConfirmed: boolean;
    gofTime: number;
}
