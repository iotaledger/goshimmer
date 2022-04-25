import { IGraph } from './graph';
import cytoscape from 'cytoscape';
import { branchDagreOptions, dagreOptions } from 'styles/graphStyle';
import { utxoVertex } from 'models/utxo';
import { branchVertex } from 'models/branch';
import { ObservableMap } from 'mobx';
import { BRANCH, LINE, UTXO } from './../styles/cytoscapeStyles';

export class cytoscapeLib implements IGraph {
    cy;
    layout;
    layoutApi;

    constructor(options: Array<any>, init: () => any) {
        options.forEach((o) => {
            cytoscape.use(o);
        });

        [this.cy, this.layout, this.layoutApi] = init();
    }

    drawVertex(data: any): void {
        this.cy.add(data);
    }

    removeVertex(id: string): void {
        const children = this.cy.getElementById(id).children();

        this.cy.remove('#' + id);
        this.cy.remove(children);
    }

    selectVertex(id: string): void {
        const node = this.cy.getElementById(id);
        if (!node) return;
        node.select();
    }

    unselectVertex(id: string): void {
        const node = this.cy.getElementById(id);
        if (!node) return;
        node.unselect();
    }

    centerVertex(id: string): void {
        const node = this.cy.getElementById(id);
        if (!node) return;
        this.cy.center(node);
    }

    centerGraph(): void {
        this.cy.center();
    }

    clearGraph(): void {
        this.cy.elements().remove();
    }

    updateLayout(): void {
        this.cy.layout(this.layout).run();
    }

    addNodeEventListener(event: string, listener: () => void): void {
        this.cy.on(event, 'node', listener);
    }
}

export function drawTransaction(
    tx: utxoVertex,
    graph: cytoscapeLib,
    outputMap: Map<any, any>
) {
    let collection = graph.cy.collection();

    // draw grouping (tx)
    collection = collection.union(
        graph.cy.add({
            group: 'nodes',
            data: { id: tx.ID },
            classes: 'transaction'
        })
    );

    // draw inputs
    const inputNodeID = tx.ID + '_input';
    let inputLabel = '';
    if (tx.inputs.length > 1) {
        inputLabel = tx.inputs.length + ' inputs';
    }
    collection = collection.union(
        graph.cy.add({
            group: 'nodes',
            data: {
                id: inputNodeID,
                parent: tx.ID,
                label: inputLabel
            },
            classes: ['input', 'center-center']
        })
    );

    tx.inputs.forEach((input) => {
        // link input to the tx that contains unspent output
        const spentOutputTx = outputMap.get(input.referencedOutputID.base58);
        if (spentOutputTx) {
            collection = collection.union(
                graph.cy.add({
                    group: 'edges',
                    data: {
                        source: spentOutputTx + '_output',
                        target: inputNodeID
                    }
                })
            );
        }
    });

    // draw outputs
    const outputNodeID = tx.ID + '_output';
    let outputLabel = '';
    if (tx.outputs.length > 1) {
        outputLabel = tx.outputs.length + ' outputs';
    }
    collection = collection.union(
        graph.cy.add({
            group: 'nodes',
            data: { id: outputNodeID, parent: tx.ID, label: outputLabel },
            classes: ['output', 'center-center']
        })
    );

    // alignment of inputs and outputs
    collection = collection.union(
        graph.cy.add({
            group: 'edges',
            data: {
                source: inputNodeID,
                target: outputNodeID
            },
            classes: 'invisible'
        })
    );

    updateConfirmedTransaction(tx, graph);
    graph.layoutApi.placeNewNodes(collection);
}

const drawSingleBranch = function (
    branch: branchVertex,
    graph: cytoscapeLib,
    branchMap: ObservableMap<string, branchVertex>
): any {
    if (!branch) {
        return;
    }
    let v: any;
    try {
        v = graph.cy.add({
            group: 'nodes',
            data: { id: branch.ID }
        });
    } catch (e) {
        // already drawn
    }
    branchMap.set(branch.ID, branch);
    updateConfirmedBranch(branch, graph);

    if (v) {
        graph.layoutApi.placeNewNodes(v);
        if (branch.isConfirmed) {
            v.addClass('confirmed');
        }
    }

    return v;
};

export async function drawBranch(
    branch: branchVertex,
    graph: cytoscapeLib,
    branchMap: ObservableMap<string, branchVertex>
) {
    if (!branch) {
        return;
    }
    drawSingleBranch(branch, graph, branchMap);
    branch.parents = branch.parents || [];
    for (let i = 0; i < branch.parents.length; i++) {
        const pID = branch.parents[i];
        const b = branchMap.get(pID);
        if (b) {
            graph.cy.add({
                group: 'edges',
                data: { source: pID, target: branch.ID }
            });
        } else {
            const res = await fetch(`/api/dagsvisualizer/branch/${pID}`);
            const branches: Array<branchVertex> =
                (await res.json()) as Array<branchVertex>;
            drawBranchesUpToMaster(branches, graph, branchMap);
            graph.cy.add({
                group: 'edges',
                data: { source: pID, target: branch.ID }
            });
        }
    }
}

function drawBranchesUpToMaster(
    branches: Array<branchVertex>,
    graph: cytoscapeLib,
    branchMap: ObservableMap<string, branchVertex>
) {
    for (let i = 0; i < branches.length; i++) {
        const branch = branches[i];
        drawSingleBranch(branch, graph, branchMap);
        branch.parents?.forEach((parentID) => {
            const parent = branches.find((b) => b.ID === parentID);
            if (parent) {
                if (!branchMap.get(parentID)) {
                    drawSingleBranch(parent, graph, branchMap);
                }
                graph.cy.add({
                    group: 'edges',
                    data: { source: parent.ID, target: branch.ID }
                });
            }
        });
    }
}

export function updateConfirmedTransaction(
    tx: utxoVertex,
    graph: cytoscapeLib
) {
    if (!tx) return;

    const node = graph.cy.getElementById(tx.ID);
    if (!node) return;
    if (tx.isConfirmed) {
        node.addClass('confirmed');
    } else if (node.hasClass('confirmed')) {
        node.removeClass('confirmed');
    }
}

export function updateConfirmedBranch(
    branch: branchVertex,
    graph: cytoscapeLib
): void {
    if (!branch)  return;

    const node = graph.cy.getElementById(branch.ID);
    if (!node) return;
    if (branch.isConfirmed) {
        node.addClass('confirmed');
    } else if (node.hasClass('confirmed')) {
        node.removeClass('confirmed');
    }
}

export function removeConfirmationStyle(id: string, graph: cytoscapeLib): void {
    const node = graph.cy.getElementById(id);
    if (!node) return;
    node.removeClass('confirmed');
}

export function initUTXODAG() {
    const cy = cytoscape({
        container: document.getElementById('utxoVisualizer'), // container to render in
        style: [
            // the stylesheet for the graph
            {
                selector: 'node',
                style: {
                    'font-weight': 'bold',
                    shape: 'rectangle',
                    width: 20,
                    height: 20
                }
            },
            {
                selector: 'edge',
                style: {
                    width: 1,
                    'curve-style': LINE.EDGE_STYLE,
                    'line-color': LINE.COLOR,
                    'control-point-step-size': '10px',
                    events: 'no'
                }
            },
            {
                selector: ':parent',
                style: {
                    'background-color': UTXO.PARENT_COLOR,
                    'min-width': '50px',
                    'min-height': '50px',
                    'border-color': UTXO.BORDER_COLOR
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'background-color': UTXO.SELECTED,
                    'border-color': UTXO.BORDER_SELECTED
                }
            },
            {
                selector: '.input',
                style: {
                    'background-color': UTXO.INPUT_COLOR,
                    'font-size': 16,
                    'padding-bottom': '2px',
                    label: 'data(label)',
                    events: 'no'
                }
            },
            {
                selector: '.output',
                style: {
                    'background-color': UTXO.OUTPUT_COLOR,
                    'font-size': 16,
                    label: 'data(label)',
                    events: 'no'
                }
            },
            {
                selector: '.confirmed',
                style: {
                    'background-color': UTXO.COLOR_CONFIRMED,
                }
            },
            {
                selector: '.invisible',
                style: {
                    visibility: 'hidden'
                }
            }
        ],
        layout: {
            name: LINE.LAYOUT
        }
    });
    const layout = dagreOptions;
    const layoutApi = cy.layoutUtilities({
        desiredAspectRatio: 1,
        polyominoGridSizeFactor: 1,
        utilityFunction: 0,
        componentSpacing: 80
    });

    return [cy, layout, layoutApi];
}

export function initBranchDAG() {
    const cy = cytoscape({
        container: document.getElementById('branchVisualizer'), // container to render in
        style: [
            // the stylesheet for the graph
            {
                selector: 'node',
                style: {
                    'background-color': BRANCH.COLOR,
                    shape: 'rectangle',
                    width: 25,
                    height: 15
                }
            },
            {
                selector: 'edge',
                style: {
                    width: 1,
                    'curve-style': LINE.EDGE_STYLE,
                    'line-color': LINE.COLOR,
                    'control-point-step-size': BRANCH.STEP,
                    events: 'no'
                }
            },
            {
                selector: 'node:selected',
                style: {
                    'background-opacity': BRANCH.OPACITY,
                    'background-color': BRANCH.SELECTED
                }
            },
            {
                selector: '.search',
                style: {}
            },
            {
                selector: '.search:selected',
                style: {
                    'background-color': BRANCH.SELECTED
                }
            },
            {
                selector: '.confirmed',
                style: {
                    'background-color': BRANCH.COLOR_CONFIRMED
                }
            }
        ],
        layout: {
            name: LINE.LAYOUT
        }
    });
    const layout = branchDagreOptions;
    const layoutApi = cy.layoutUtilities({
        desiredAspectRatio: 1,
        polyominoGridSizeFactor: 1,
        utilityFunction: 0,
        componentSpacing: 200
    });

    return [cy, layout, layoutApi];
}
