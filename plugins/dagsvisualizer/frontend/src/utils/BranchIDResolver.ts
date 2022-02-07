export function resolveBase58BranchID(base58Branch: string): string {
    switch (base58Branch) {
    case MasterBranchInBase58:
        return 'MasterBranchID';
    case UndefinedBranchInBase58:
        return 'UndefinedBranchID';
    case LazyBookedConflictsBranchInBase58:
        return 'LazyBookedConflictsBranchID';
    case InvalidBranchInBase58:
        return 'InvalidBranchID';
    default:
        // otherwise it is a "regular" branchID that doesn't have a distinct name
        return base58Branch;
    }
}

// base58 branchIDs that have distinct names
const MasterBranchInBase58 = '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM';
const UndefinedBranchInBase58 = '11111111111111111111111111111111';
const LazyBookedConflictsBranchInBase58 =
    'JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFF';
const InvalidBranchInBase58 = 'JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFG';
