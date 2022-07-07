export function resolveBase58ConflictID(base58Conflict: string): string {
    switch (base58Conflict) {
    case MasterConflictInBase58:
        return 'MasterConflictID';
    case UndefinedConflictInBase58:
        return 'UndefinedConflictID';
    case LazyBookedConflictsConflictInBase58:
        return 'LazyBookedConflictsConflictID';
    case InvalidConflictInBase58:
        return 'InvalidConflictID';
    default:
        // otherwise it is a "regular" conflictID that doesn't have a distinct name
        return base58Conflict;
    }
}

// base58 conflictIDs that have distinct names
const MasterConflictInBase58 = '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM';
const UndefinedConflictInBase58 = '11111111111111111111111111111111';
const LazyBookedConflictsConflictInBase58 =
    'JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFF';
const InvalidConflictInBase58 = 'JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFG';
