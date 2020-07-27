export function parseColor(color: number | string): number {
    if (typeof color === "number") {
        return color;
    }

    let parsedColor: number = 0;

    if (typeof color === "string") {
        if (color.length === 4) {
            // #rgb, duplicate each letter except first #.
            color = color.replace(/([^#])/g, "$1$1");
        }
        if (color.length === 9) {
            // #rrggbbaa
            parsedColor = parseInt(color.substr(1), 16);
        } else if (color.length === 7) {
            // or #rrggbb.
            parsedColor = parseInt(color.substr(1), 16) << 8 | 0xff;
        } else {
            throw new Error(`Color expected in hex format with preceding "#". E.g. #00ff00. Got value: ${color}`);
        }
    }

    return parsedColor;
}
