import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from '../WS';
import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import { dagreOptions } from 'styles/graphStyle';
import layoutUtilities from 'cytoscape-layout-utilities';
import 'styles/style.css';

export class utxoVertex {
    msgID: string;
    ID: string;
    inputs: Array<input>;
    outputs: Array<string>;
    branchID: string;
    isConfirmed: boolean;
    gof: string;
    confirmedTime: number;
}

export class input {
    type: string;
    referencedOutputID: any;
}

export class utxoBooked {
    ID: string;
    branchID: string;
}

export class utxoConfirmed {
    ID: string;
    gof: string;
    confirmedTime: number;
}

export class UTXOStore {
    @observable maxUTXOVertices = 500;
    @observable transactions = new ObservableMap<string, utxoVertex>();
    @observable selectedTx: utxoVertex = null;
    @observable paused = false;
    @observable search = '';
    @observable explorerAddress = 'localhost:8081';
    outputMap = new Map();
    txOrder: Array<any> = [];
    highligtedTxs = [];
    draw = true;

    vertexChanges = 0;
    txToRemoveAfterResume = [];
    txToAddAfterResume = [];

    cy;
    layoutUpdateTimerID;
    layout;
    layoutApi;

    constructor() {
        makeObservable(this);
        registerHandler(WSMsgType.Transaction, this.addTransaction);
        registerHandler(WSMsgType.TransactionBooked, this.setTxBranch);
        registerHandler(
            WSMsgType.TransactionConfirmed,
            this.setTXConfirmedTime
        );

        cytoscape.use(dagre);
        cytoscape.use(layoutUtilities);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Transaction);
        unregisterHandler(WSMsgType.TransactionConfirmed);
    }

    @action
    addTransaction = (tx: utxoVertex) => {
        if (this.txOrder.length >= this.maxUTXOVertices) {
            const removed = this.txOrder.shift();
            const txObj = this.transactions.get(removed);
            txObj.outputs.forEach(output => {
                this.outputMap.delete(output);
            });
            this.transactions.delete(removed);

            if (this.paused) {
                // keep the removed tx that should be removed from the graph after resume.
                this.txToRemoveAfterResume.push(removed);
            } else {
                this.removeVertex(removed);
            }
        }

        this.txOrder.push(tx.ID);
        tx.branchID = '';
        this.transactions.set(tx.ID, tx);
        tx.outputs.forEach(outputID => {
            this.outputMap.set(outputID, {});
        });

        if (this.paused) {
            this.txToAddAfterResume.push(tx.ID);
            return;
        }
        if (this.draw) {
            this.drawVertex(tx);
        }
    };

    @action
    setTxBranch = (bookedTx: utxoBooked) => {
        const tx = this.transactions.get(bookedTx.ID);
        if (!tx) {
            return;
        }

        tx.branchID = bookedTx.branchID;
        this.transactions.set(bookedTx.ID, tx);
    };

    @action
    setTXConfirmedTime = (txConfirmed: utxoConfirmed) => {
        const tx = this.transactions.get(txConfirmed.ID);
        if (!tx) {
            return;
        }

        tx.isConfirmed = true;
        tx.confirmedTime = txConfirmed.confirmedTime;
        tx.gof = txConfirmed.gof;
        this.transactions.set(txConfirmed.ID, tx);
    };

    @action
    updateSelected = (txID: string) => {
        const tx = this.transactions.get(txID);
        if (!tx) return;
        this.selectedTx = tx;
    };

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
        // unselect preselected node manually
        if (removePreSelectedNode && this.selectedTx) {
            this.cy.getElementById(this.selectedTx.ID).unselect();
        }

        this.selectedTx = null;
    };

    @action
    pauseResume = () => {
        if (this.paused) {
            this.resumeAndSyncGraph();
            this.paused = false;
            return;
        }
        this.paused = true;
    };

    @action
    updateVerticesLimit = (num: number) => {
        this.maxUTXOVertices = num;
    };

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    };

    @action
    searchAndSelect = () => {
        if (!this.search) return;

        this.selectTx(this.search);
    };

    selectTx = (txID: string) => {
        // clear pre-selected node first.
        this.clearSelected(true);

        this.highlightTx(txID);
        this.centerTx(txID);

        this.updateSelected(this.search);
    };

    highlightTxs = (txIDs: string[]) => {
        this.highligtedTxs.forEach(id => {
            this.clearHighlightedTx(id);
        });

        // update highlighted msgs
        this.highligtedTxs = txIDs;
        txIDs.forEach(id => {
            this.highlightTx(id);
        });
    };

    highlightTx = (txID: string) => {
        const txNode = this.cy.getElementById(txID);
        if (!txNode) return;
        txNode.select();
    };

    centerTx = (txID: string) => {
        const txNode = this.cy.getElementById(txID);
        if (!txNode) return;
        this.cy.center(txNode);
    };

    clearHighlightedTx = (txID: string) => {
        const txNode = this.cy.getElementById(txID);
        if (!txNode) return;
        txNode.unselect();
    };

    clearHighlightedTxs = () => {
        this.highligtedTxs.forEach(id => {
            this.clearHighlightedTx(id);
        });
    };

    getTxsFromBranch = (branchID: string) => {
        const txs = [];
        this.transactions.forEach((tx: utxoVertex) => {
            if (tx.branchID === branchID) {
                txs.push(tx.ID);
            }
        });

        return txs;
    };

    updateExplorerAddress = (addr: string) => {
        this.explorerAddress = addr;
    };

    resumeAndSyncGraph = () => {
        // add buffered tx
        this.txToAddAfterResume.forEach(txID => {
            const tx = this.transactions.get(txID);
            if (tx) {
                this.drawVertex(tx);
            }
        });
        this.txToAddAfterResume = [];

        // remove removed tx
        this.txToRemoveAfterResume.forEach(txID => {
            this.removeVertex(txID);
        });
        this.txToRemoveAfterResume = [];
    };

    drawExistedTxs = () => {
        this.transactions.forEach(tx => {
            this.drawVertex(tx);
        });
    };

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    };

    clearGraph = () => {
        this.cy.elements().remove();
    };

    centerEntireGraph = () => {
        this.cy.center();
    };

    removeVertex = (txID: string) => {
        const children = this.cy.getElementById(txID).children();

        this.cy.remove('#' + txID);
        this.cy.remove(children);
        this.vertexChanges++;
    };

    drawVertex = (tx: utxoVertex) => {
        this.vertexChanges++;
        let collection = this.cy.collection();

        // draw grouping (tx)
        collection = collection.union(
            this.cy.add({
                group: 'nodes',
                data: { id: tx.ID },
                classes: 'transaction'
            })
        );

        // draw inputs
        const inputIDs = [];
        tx.inputs.forEach((input, index) => {
            // input node
            const inputNodeID = hashString(
                input.referencedOutputID.base58 + tx.ID + '_input'
            );
            collection = collection.union(
                this.cy.add({
                    group: 'nodes',
                    data: {
                        id: inputNodeID,
                        parent: tx.ID,
                        input: input.referencedOutputID.base58
                    },
                    classes: 'input'
                })
            );

            // align every 5 inputs in the same level
            if (index >= 5) {
                collection = collection.union(
                    this.cy.add({
                        group: 'edges',
                        data: {
                            source: inputIDs[index - 5],
                            target: inputNodeID
                        },
                        classes: 'invisible'
                    })
                );
            }
            inputIDs.push(inputNodeID);

            // link input to the unspent output
            const spentOutput = this.outputMap.get(
                input.referencedOutputID.base58
            );
            if (spentOutput) {
                collection = collection.union(
                    this.cy.add({
                        group: 'edges',
                        data: {
                            source: input.referencedOutputID.base58,
                            target: inputNodeID
                        }
                    })
                );
            }
        });

        // draw outputs
        const outputIDs = [];
        tx.outputs.forEach((outputID, index) => {
            collection = collection.union(
                this.cy.add({
                    group: 'nodes',
                    data: { id: outputID, parent: tx.ID },
                    classes: 'output'
                })
            );

            // align every 5 outputs in the same level
            if (index >= 5) {
                collection = collection.union(
                    this.cy.add({
                        group: 'edges',
                        data: {
                            source: outputIDs[index - 5],
                            target: outputID
                        },
                        classes: 'invisible'
                    })
                );
            }
            outputIDs.push(outputID);
        });

        // alignment of inputs and outputs
        const inIndex =
            Math.floor(inputIDs.length / 5) * 5 + inputIDs.length % 5 - 1;
        const outIndex = Math.min(outputIDs.length, 2);
        collection = collection.union(
            this.cy.add({
                group: 'edges',
                data: {
                    source: inputIDs[inIndex],
                    target: outputIDs[outIndex]
                },
                classes: 'invisible'
            })
        );

        this.layoutApi.placeNewNodes(collection);
    };

    updateLayoutTimer = () => {
        this.layoutUpdateTimerID = setInterval(() => {
            if (this.vertexChanges > 0 && !this.paused) {
                this.cy.layout(this.layout).run();
                this.vertexChanges = 0;
                console.log('layout updated');
            }
        }, 10000);
    };

    start = () => {
        this.cy = cytoscape({
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
                        'curve-style': 'bezier',
                        'line-color': '#696969',
                        'control-point-step-size': '10px',
                        events: 'no'
                    }
                },
                {
                    selector: ':parent',
                    style: {
                        'background-opacity': 0.333,
                        'background-color': '#15B5B0',
                        'min-width': '50px',
                        'min-height': '50px'
                    }
                },
                {
                    selector: 'node:selected',
                    style: {
                        'background-opacity': 0.333,
                        'background-color': 'red'
                    }
                },
                {
                    selector: '.input',
                    style: {
                        'background-color': '#F9BDC0',
                        events: 'no'
                    }
                },
                {
                    selector: '.output',
                    style: {
                        'background-color': '#FBE698',
                        events: 'no'
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
                name: 'dagre'
            }
        });
        this.layout = dagreOptions;
        this.layoutApi = this.cy.layoutUtilities({
            desiredAspectRatio: 1,
            polyominoGridSizeFactor: 1,
            utilityFunction: 0,
            componentSpacing: 80
        });

        // set up click event
        this.cy.on('select', 'node', evt => {
            const node = evt.target;
            const nodeData = node.json();

            this.updateSelected(nodeData.data.id);
        });

        // clear selected node
        this.cy.on('unselect', 'node', () => {
            this.clearSelected();
        });

        this.updateLayoutTimer();
    };

    stop = () => {
        this.unregisterHandlers();

        // stop updating layout.
        clearInterval(this.layoutUpdateTimerID);
        // maybe store graph history?
    };
}

export default UTXOStore;

function hashString(source: string) {
    let hash = 0;
    if (source.length === 0) {
        return hash;
    }
    for (let i = 0; i < source.length; i++) {
        const char = source.charCodeAt(i);
        hash = (hash << 5) - hash + char;
        hash = hash & hash; // Convert to 32bit integer
    }
    return hash.toString();
}
