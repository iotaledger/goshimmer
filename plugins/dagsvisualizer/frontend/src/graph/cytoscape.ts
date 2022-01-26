import { IGraph } from './graph';
import cytoscape from 'cytoscape';
import { dagreOptions } from 'styles/graphStyle';
import { utxoVertex } from 'models/utxo';
import { hashString } from 'utils/hashString';
import { branchVertex } from 'models/branch';
import { ObservableMap } from 'mobx';

export class cytoscapeLib implements IGraph {
  cy;
  layout;
  layoutApi;
  branchAPICache: Map<string, branchVertex>;

  constructor(options: Array<any>, init: () => any) {
      options.forEach((o) => {
          cytoscape.use(o);
      });

      [this.cy, this.layout, this.layoutApi] = init();

      this.branchAPICache = new Map();
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
            classes: 'transaction',
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

    tx.inputs.forEach(input => {
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
            classes: 'invisible',
        })
    );

    graph.layoutApi.placeNewNodes(collection);
}

export async function drawBranch(
    branch: branchVertex,
    graph: cytoscapeLib,
    branchMap: ObservableMap<string, branchVertex>
) {
    if (!branch) {
        return;
    }
    let v: any;
    try {
        v = graph.cy.add({
            group: 'nodes',
            data: { id: branch.ID },
        });
    } catch (e) {
        // already exists. never mind
    } finally {
        branchMap.set(branch.ID, branch);
    }

    branch.parents = branch.parents || [];
    for (let i = 0; i < branch.parents.length; i++) {
        const pID = branch.parents[i];
        const b = branchMap.get(pID);
        if (b) {
            graph.cy.add({
                group: 'edges',
                data: { source: pID, target: branch.ID },
            });
        } else {
            // recursively fetch branch and draw parent
            const res = await fetch(`/api/dagsvisualizer/branch/${pID}`);
            const vertex: branchVertex = (await res.json()) as branchVertex;
            console.log('parent found: ', vertex);
            await drawBranch(vertex, graph, branchMap);
            // make sure the parent was added
            if (branchMap.get(pID)) {
                graph.cy.add({
                    group: 'edges',
                    data: { source: pID, target: branch.ID },
                });
            }
        }
    }

    if (v) {
        graph.layoutApi.placeNewNodes(v);
    }
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
                    height: 20,
                },
            },
            {
                selector: 'edge',
                style: {
                    width: 1,
                    'curve-style': 'bezier',
                    'line-color': '#696969',
                    'control-point-step-size': '10px',
                    events: 'no',
                },
            },
            {
                selector: ':parent',
                style: {
                    'background-opacity': 0.333,
                    'background-color': '#15B5B0',
                    'min-width': '50px',
                    'min-height': '50px',
                },
            },
            {
                selector: 'node:selected',
                style: {
                    'background-opacity': 0.333,
                    'background-color': 'red',
                },
            },
            {
                selector: '.input',
                style: {
                    'background-color': '#F9BDC0',
                    'font-size': 16,
                    label: 'data(label)',
                    events: 'no'
                }
            },
            {
                selector: '.output',
                style: {
                    'background-color': '#FBE698',
                    'font-size': 16,
                    label: 'data(label)',
                    events: 'no'
                }
            },
            {
                selector: '.invisible',
                style: {
                    visibility: 'hidden',
                },
            },
        ],
        layout: {
            name: 'dagre',
        },
    });
    const layout = dagreOptions;
    const layoutApi = cy.layoutUtilities({
        desiredAspectRatio: 1,
        polyominoGridSizeFactor: 1,
        utilityFunction: 0,
        componentSpacing: 80,
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
                    'background-color': '#2E8BC0',
                    shape: 'rectangle',
                    width: 25,
                    height: 15,
                },
            },
            {
                selector: 'edge',
                style: {
                    width: 1,
                    'curve-style': 'bezier',
                    'line-color': '#696969',
                    'control-point-step-size': '10px',
                    events: 'no',
                },
            },
            {
                selector: 'node:selected',
                style: {
                    'background-opacity': 0.333,
                    'background-color': 'red',
                },
            },
            {
                selector: '.search',
                style: {
                    'background-color': 'yellow',
                },
            },
        ],
        layout: {
            name: 'dagre',
        },
    });
    const layout = dagreOptions;
    const layoutApi = cy.layoutUtilities({
        desiredAspectRatio: 1,
        polyominoGridSizeFactor: 1,
        utilityFunction: 0,
        componentSpacing: 200,
    });

    return [cy, layout, layoutApi];
}
