import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSBlkType } from 'utils/WS';
import { MAX_VERTICES } from 'utils/constants';
import dagre from 'cytoscape-dagre';
import layoutUtilities from 'cytoscape-layout-utilities';
import 'styles/style.css';
import {
    cytoscapeLib,
    drawTransaction,
    initUTXODAG,
    removeConfirmationStyle,
    updateConfirmedTransaction
} from 'graph/cytoscape';
import { utxoBooked, utxoConfirmationStateChanged, utxoVertex } from 'models/utxo';

export class UTXOStore {
    @observable maxUTXOVertices = MAX_VERTICES;
    @observable transactions = new ObservableMap<string, utxoVertex>();
    @observable foundTxs = new ObservableMap<string, utxoVertex>();
    @observable selectedTx: utxoVertex = null;
    @observable paused = false;
    @observable search = '';
    foundOutputMap = new Map();
    outputMap = new Map();
    txOrder: Array<any> = [];
    highlightedTxs = [];
    draw = true;

    vertexChanges = 0;
    txToRemoveAfterResume = [];
    txToAddAfterResume = [];

    layoutUpdateTimerID;

    graph;

    constructor() {
        makeObservable(this);
        registerHandler(WSBlkType.Transaction, this.addTransaction);
        registerHandler(WSBlkType.TransactionBooked, this.setTxConflict);
        registerHandler(
            WSBlkType.TransactionConfirmationStateChanged,
            this.transactionConfirmationStateChanged
        );
    }

    unregisterHandlers() {
        unregisterHandler(WSBlkType.Transaction);
        unregisterHandler(WSBlkType.TransactionBooked);
        unregisterHandler(WSBlkType.TransactionConfirmationStateChanged);
    }

    @action
    addTransaction = (tx: utxoVertex) => {
        this.checkLimit();

        this.txOrder.push(tx.ID);
        this.transactions.set(tx.ID, tx);
        tx.outputs.forEach((outputID) => {
            this.outputMap.set(outputID, tx.ID);
        });

        if (this.paused) {
            this.txToAddAfterResume.push(tx.ID);
            return;
        }
        if (this.draw) {
            this.drawVertex(tx);
        }
    };

    checkLimit = () => {
        if (this.txOrder.length >= this.maxUTXOVertices) {
            const removed = this.txOrder.shift();
            const txObj = this.transactions.get(removed);
            txObj.outputs.forEach((output) => {
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
    };

    @action
    addFoundTx = (tx: utxoVertex) => {
        this.foundTxs.set(tx.ID, tx);
        tx.outputs.forEach((outputID) => {
            this.foundOutputMap.set(outputID, tx.ID);
        });
    };

    @action
    clearFoundTxs = () => {
        this.foundTxs.clear();
        this.foundOutputMap.clear();
    };

    @action
    setTxConflict = (bookedTx: utxoBooked) => {
        const tx = this.transactions.get(bookedTx.ID);
        if (!tx) {
            return;
        }

        tx.conflictID = bookedTx.conflictID;
        this.transactions.set(bookedTx.ID, tx);
    };

    @action transactionConfirmationStateChanged = (txConfirmationState: utxoConfirmationStateChanged) => {
        this.setTXConfirmationStateTime(txConfirmationState);
        this.updateUTXO(txConfirmationState);
    };

    @action
    setTXConfirmationStateTime = (txConfirmationState: utxoConfirmationStateChanged) => {
        const tx = this.transactions.get(txConfirmationState.ID);
        if (!tx) {
            return;
        }

        if (txConfirmationState.isConfirmed) {
            tx.isConfirmed = true;
        } else {
            tx.isConfirmed = false;
        }

        tx.confirmationStateTime = txConfirmationState.confirmationStateTime;
        tx.confirmationState = txConfirmationState.confirmationState;
        this.transactions.set(txConfirmationState.ID, tx);
    };

    @action
    updateSelected = (txID: string) => {
        const tx = this.transactions.get(txID) || this.foundTxs.get(txID);
        if (!tx) return;
        this.selectedTx = tx;
        removeConfirmationStyle(txID, this.graph);
    };

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
        // unselect preselected node manually
        if (removePreSelectedNode && this.selectedTx) {
            this.graph.unselectVertex(this.selectedTx.ID);
        }
        if (this.selectedTx) {
            updateConfirmedTransaction(this.selectedTx, this.graph);
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
        this.trimTxToVerticesLimit();
    };

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    };

    @action
    searchAndSelect = () => {
        if (!this.search) return;

        this.selectTx(this.search);
        this.centerTx(this.search);
    };

    selectTx = (txID: string) => {
        // clear pre-selected node first.
        this.clearSelected(true);
        this.graph.selectVertex(txID);
        this.updateSelected(txID);
    };

    getTxsFromConflict = (conflictID: string, searchMode: boolean) => {
        const txs = [];

        if (searchMode) {
            this.foundTxs.forEach((tx: utxoVertex) => {
                if (tx.conflictID === conflictID) {
                    txs.push(tx.ID);
                }
            });

            return txs;
        }

        this.transactions.forEach((tx: utxoVertex) => {
            if (tx.conflictID === conflictID) {
                txs.push(tx.ID);
            }
        });

        return txs;
    };

    resumeAndSyncGraph = () => {
        // add buffered tx
        this.txToAddAfterResume.forEach((txID) => {
            const tx = this.transactions.get(txID);
            if (tx) {
                this.drawVertex(tx);
            }
        });
        this.txToAddAfterResume = [];

        // remove removed tx
        this.txToRemoveAfterResume.forEach((txID) => {
            this.removeVertex(txID);
        });
        this.txToRemoveAfterResume = [];
    };

    drawExistedTxs = () => {
        this.transactions.forEach((tx) => {
            this.drawVertex(tx);
        });
    };

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    };

    drawFoundVertex = (tx: utxoVertex) => {
        drawTransaction(tx, this.graph, this.foundOutputMap);
        this.vertexChanges++;
    };

    drawVertex = (tx: utxoVertex) => {
        drawTransaction(tx, this.graph, this.outputMap);
        updateConfirmedTransaction(tx, this.graph);
        this.vertexChanges++;
    };

    removeVertex = (txID: string) => {
        this.graph.removeVertex(txID);
        this.vertexChanges++;
    };

    highlightTxs = (txIDs: string[]) => {
        this.clearHighlightedTxs();

        // update highlighted txs
        this.highlightedTxs = txIDs;
        txIDs.forEach((id) => {
            this.graph.selectVertex(id);
        });
    };

    clearHighlightedTxs = () => {
        this.highlightedTxs.forEach((id) => {
            this.graph.unselectVertex(id);
        });
    };

    centerTx = (txID: string) => {
        this.graph.centerVertex(txID);
    };

    centerEntireGraph = () => {
        this.graph.centerGraph();
    };

    clearGraph = () => {
        this.graph.clearGraph();
    };

    updateLayoutTimer = () => {
        this.layoutUpdateTimerID = setInterval(() => {
            if (this.vertexChanges > 0 && !this.paused) {
                this.graph.updateLayout();
                this.vertexChanges = 0;
            }
        }, 10000);
    };

    trimTxToVerticesLimit() {
        if (this.txOrder.length >= this.maxUTXOVertices) {
            const removeStartIndex = this.txOrder.length - this.maxUTXOVertices;
            const removed = this.txOrder.slice(0, removeStartIndex);
            this.txOrder = this.txOrder.slice(removeStartIndex);
            this.removeTxs(removed);
        }
    }

    removeTxs(removed: string[]) {
        removed.forEach((id: string) => {
            const t = this.transactions.get(id);
            if (t) {
                this.removeVertex(id);
                t.outputs.forEach((output) => {
                    this.outputMap.delete(output);
                });
                this.transactions.delete(id);
            }
        });
    }

    updateUTXO(utxoConfirmationState: utxoConfirmationStateChanged) {
        const tx = this.transactions.get(utxoConfirmationState.ID);
        if (tx) {
            updateConfirmedTransaction(tx, this.graph);
        }
    }

    start = () => {
        this.graph = new cytoscapeLib([dagre, layoutUtilities], initUTXODAG);

        // set up click event
        this.graph.addNodeEventListener('select', (evt) => {
            const node = evt.target;
            const nodeData = node.json();

            this.updateSelected(nodeData.data.id);
        });

        // clear selected node
        this.graph.addNodeEventListener('unselect', () => {
            this.clearSelected();
        });

        this.updateLayoutTimer();
    };

    stop = () => {
        this.unregisterHandlers();

        // stop updating layout.
        clearInterval(this.layoutUpdateTimerID);
    };
}

export default UTXOStore;
