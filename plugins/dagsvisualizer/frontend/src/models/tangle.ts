export class tangleVertex {
    ID: string;
    strongParentIDs: Array<string>;
    weakParentIDs: Array<string>;
    shallowLikeParentIDs: Array<string>;
    conflictIDs: Array<string>;
    isMarker: boolean;
    isTx: boolean;
    txID: string;
    isTip: boolean;
    isConfirmed: boolean;
    isTxConfirmed: boolean;
    confirmationState: string;
    confirmationStateTime: number;
}

export class tangleBooked {
    ID: string;
    isMarker: boolean;
    conflictIDs: Array<string>;
}

export class tangleConfirmed {
    ID: string;
    confirmationState: string;
    confirmationStateTime: number;
}

export class tangleTxConfirmationStateChanged {
    ID: string;
    isConfirmed: boolean;
}

export enum parentRefType {
    StrongRef,
    WeakRef,
    ShallowLikeRef,
    ShallowDislikeRef
}
