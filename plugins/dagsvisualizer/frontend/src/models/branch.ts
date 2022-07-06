export class branchVertex {
    ID: string;
    parents: Array<string>;
    isConfirmed: boolean;
    conflicts: conflictBranches;
    confirmationState: string;
    aw: number;
}

export class conflictBranches {
    branchID: string;
    conflicts: Array<conflict>;
}

export class conflict {
    outputID: any;
    branchIDs: Array<string>;
}
export class branchParentUpdate {
    ID: string;
    parents: Array<string>;
}

export class branchConfirmationStateChanged {
    ID: string;
    confirmationState: string;
    isConfirmed: boolean;
}

export class branchWeightChanged {
    ID: string;
    weight: number;
    confirmationState: string;
}
