export class conflictVertex {
    ID: string;
    parents: Array<string>;
    isConfirmed: boolean;
    conflicts: conflictConflicts;
    confirmationState: string;
    aw: number;
}

export class conflictConflicts {
    conflictID: string;
    conflicts: Array<conflict>;
}

export class conflict {
    outputID: any;
    conflictIDs: Array<string>;
}
export class conflictParentUpdate {
    ID: string;
    parents: Array<string>;
}

export class conflictConfirmationStateChanged {
    ID: string;
    confirmationState: string;
    isConfirmed: boolean;
}

export class conflictWeightChanged {
    ID: string;
    weight: number;
    confirmationState: string;
}
