export enum ConfirmationState {
    Undefined = 0,
    Rejected,
    Pending,
    Accepted,
    Confirmed,
}

export function resolveConfirmationState(sigType: number) {
    switch (sigType) {
        case ConfirmationState.Undefined:
            return "Undefined";
        case ConfirmationState.Rejected:
            return "Rejected";
        case ConfirmationState.Pending:
            return "Pending";
        case ConfirmationState.Accepted:
            return "Accepted";
        case ConfirmationState.Confirmed:
            return "Confirmed";
        default:
            return "Undefined Confirmation State";
    }
}
