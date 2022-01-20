export interface IGraph {
    drawVertex(data: any): void;
    removeVertex(id: string): void;
    selectVertex(id: string): void;
    unselectVertex(id: string): void;
    centerVertex(id: string): void;
    centerGraph(): void;
    clearGraph(): void;
}
