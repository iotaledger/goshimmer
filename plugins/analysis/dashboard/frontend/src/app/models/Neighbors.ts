export class Neighbors {
    in: Set<string>;
    out: Set<string>;

    constructor() {
        this.in = new Set();
        this.out = new Set();
    }
}