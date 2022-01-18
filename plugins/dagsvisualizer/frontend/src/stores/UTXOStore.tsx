import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from 'utils/WS';
import { MAX_VERTICES } from 'utils/constants';
import dagre from 'cytoscape-dagre';
import layoutUtilities from 'cytoscape-layout-utilities';
import 'styles/style.css';
import { cytoscapeLib, drawTransaction, initUTXODAG } from 'graph/cytoscape';
import { utxoVertex, utxoBooked, utxoConfirmed } from 'models/utxo';

export class UTXOStore {
    @observable maxUTXOVertices = MAX_VERTICES;
    @observable transactions = new ObservableMap<string, utxoVertex>();
    @observable selectedTx: utxoVertex = null;
    @observable paused = false;
    @observable search = '';
    outputMap = new Map();
    txOrder: Array<any> = [];
    highligtedTxs = [];
    draw = true;

    vertexChanges = 0;
    txToRemoveAfterResume = [];
    txToAddAfterResume = [];

    layoutUpdateTimerID;

    graph;

    constructor() {
        makeObservable(this);
        registerHandler(WSMsgType.Transaction, this.addTransaction);
        registerHandler(WSMsgType.TransactionBooked, this.setTxBranch);
        registerHandler(
            WSMsgType.TransactionConfirmed,
            this.setTXConfirmedTime
        );
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Transaction);
        unregisterHandler(WSMsgType.TransactionConfirmed);
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
            this.graph.unselectVertex(this.selectedTx.ID);
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

        this.graph.selectVertex(txID);

        this.updateSelected(this.search);
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

    drawVertex = (tx: utxoVertex) => {
        drawTransaction(tx, this.graph, this.outputMap);
        this.vertexChanges++;
    };

    removeVertex = (txID: string) => {
        this.graph.removeVertex(txID);
        this.vertexChanges++;
    };

    highlightTxs = (txIDs: string[]) => {
        this.clearHighlightedTxs();

        // update highlighted msgs
        this.highligtedTxs = txIDs;
        txIDs.forEach(id => {
            this.graph.selectVertex(id);
        });
    };

    clearHighlightedTxs = () => {
        this.highligtedTxs.forEach(id => {
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
                console.log('layout updated');
            }
        }, 10000);
    };

    start = () => {
        this.graph = new cytoscapeLib([dagre, layoutUtilities], initUTXODAG);

        // set up click event
        this.graph.cy.on('select', 'node', evt => {
            const node = evt.target;
            const nodeData = node.json();

            this.updateSelected(nodeData.data.id);
        });

        // clear selected node
        this.graph.cy.on('unselect', 'node', () => {
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
