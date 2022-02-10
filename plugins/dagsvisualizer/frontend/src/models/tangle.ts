export class tangleVertex {
    ID: string;
    strongParentIDs: Array<string>;
    weakParentIDs: Array<string>;
    likedParentIDs: Array<string>;
    branchID: string;
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
    branchID: string;
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
    LikedRef
}
