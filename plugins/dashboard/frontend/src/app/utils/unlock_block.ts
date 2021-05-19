export function resolveSignatureType(sigType: number) {
    switch (sigType) {
        case 0:
            return "Ed25519 Signature";
        case 1:
            return "BLS Signature";
        default:
            return "Unknown Signature Type";
    }
}