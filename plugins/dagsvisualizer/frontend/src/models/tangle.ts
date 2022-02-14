export class tangleVertex {
    ID: string;
    strongParentIDs: Array<string>;
    weakParentIDs: Array<string>;
    shallowLikeParentIDs: Array<string>;
    shallowDislikeParentIDs: Array<string>;
    branchIDs: Array<string>;
    isMarker: boolean;
    isTx: boolean;
    txID: string;
    isTip: boolean;
    isConfirmed: boolean;
    gof: string;
    confirmedTime: number;
    futureMarkers: Array<string>;
}

export class tangleBooked {
    ID: string;
    isMarker: boolean;
    branchIDs: Array<string>;
}

export class tangleConfirmed {
    ID: string;
    gof: string;
    confirmedTime: number;
}

export class tangleFutureMarkerUpdated {
    ID: string;
    futureMarkerID: string;
}

export enum parentRefType {
    StrongRef,
    WeakRef,
    ShallowLikeRef,
    ShallowDislikeRef
}
