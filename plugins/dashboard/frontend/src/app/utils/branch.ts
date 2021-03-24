export function resolveBase58BranchID(base58Branch: string): string {
    switch (base58Branch) {
        case MasterBranchInBase58:
            return "MasterBranchID";
        case UndefinedBranchID:
            return "UndefinedBranchID";
        case LazyBookedConflictsBranchID:
            return "LazyBookedConflictsBranchID";
        case InvalidBranchID:
            return "InvalidBranchID";
        default:
            // otherwise it is a "regular" branchID that doesn't have a distinct name
            return base58Branch
    }
}

const MasterBranchInBase58 = "4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM"
const UndefinedBranchID = "11111111111111111111111111111111"
const LazyBookedConflictsBranchID = "JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFF"
const InvalidBranchID = "JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFG"