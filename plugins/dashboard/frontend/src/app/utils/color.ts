export function resolveColor(color: string) {
    switch (color) {
        case Base58EncodedColorIOTA:
            return "IOTA";
        case Base58EncodedColorMint:
            return "MINT";
        default:
            return color;
    }
}

export const Base58EncodedColorIOTA = "11111111111111111111111111111111"
export const Base58EncodedColorMint = "JEKNVnkbo3jma5nREBBJCDoXFVeKkD56V3xKrvRmWxFG"