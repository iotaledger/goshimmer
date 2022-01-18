import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from 'utils/WS';
import { MAX_VERTICES } from 'utils/constants';
import dagre from 'cytoscape-dagre';
import layoutUtilities from 'cytoscape-layout-utilities';
import { cytoscapeLib, drawBranch, initBranchDAG } from 'graph/cytoscape';
import {
    branchVertex,
    branchParentUpdate,
    branchConfirmed,
    branchWeightChanged
} from 'models/branch';

export class BranchStore {
    @observable maxBranchVertices = MAX_VERTICES;
    @observable branches = new ObservableMap<string, branchVertex>();
    @observable selectedBranch: branchVertex = null;
    @observable paused = false;
    @observable search = '';
    branchOrder: Array<any> = [];
    draw = true;

    vertexChanges = 0;
    branchToRemoveAfterResume = [];
    branchToAddAfterResume = [];

    layoutUpdateTimerID;
    graph;

    constructor() {
        makeObservable(this);
        registerHandler(WSMsgType.Branch, this.addBranch);
        registerHandler(WSMsgType.BranchParentsUpdate, this.updateParents);
        registerHandler(WSMsgType.BranchConfirmed, this.branchConfirmed);
        registerHandler(
            WSMsgType.BranchWeightChanged,
            this.branchWeightChanged
        );
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Branch);
        unregisterHandler(WSMsgType.BranchParentsUpdate);
        unregisterHandler(WSMsgType.BranchConfirmed);
        unregisterHandler(WSMsgType.BranchWeightChanged);
    }

    @action
    addBranch = (branch: branchVertex) => {
        if (this.branchOrder.length >= this.maxBranchVertices) {
            const removed = this.branchOrder.shift();
            this.branches.delete(removed);

            if (this.paused) {
                // keep the removed tx that should be removed from the graph after resume.
                this.branchToRemoveAfterResume.push(removed);
            } else {
                this.removeVertex(removed);
            }
        }

        this.branchOrder.push(branch.ID);
        this.branches.set(branch.ID, branch);

        if (this.paused) {
            this.branchToAddAfterResume.push(branch.ID);
        }
        if (this.draw) {
            this.drawVertex(branch);
        }
    };

    @action
    updateParents = (newParents: branchParentUpdate) => {
        const b = this.branches.get(newParents.ID);
        if (!b) {
            return;
        }

        b.parents = newParents.parents;
        this.branches.set(newParents.ID, b);
    };

    @action
    branchConfirmed = (confirmedBranch: branchConfirmed) => {
        const b = this.branches.get(confirmedBranch.ID);
        if (!b) {
            return;
        }

        b.isConfirmed = true;
        this.branches.set(confirmedBranch.ID, b);
    };

    @action
    branchWeightChanged = (branch: branchWeightChanged) => {
        const b = this.branches.get(branch.ID);
        if (!b) {
            return;
        }
        b.aw = branch.weight;
        b.gof = branch.gof;
        this.branches.set(branch.ID, b);
    };

    @action
    updateSelected = (branchID: string) => {
        const b = this.branches.get(branchID);
        if (!b) return;
        this.selectedBranch = b;
    };

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
        // unselect preselected node manually
        if (removePreSelectedNode && this.selectedBranch) {
            this.graph.unselectVertex(this.selectedBranch.ID);
        }

        this.selectedBranch = null;
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
        this.maxBranchVertices = num;
    };

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    };

    @action
    searchAndHighlight = () => {
        if (!this.search) return;

        this.selectBranch(this.search);
    };

    getBranchVertex = (branchID: string) => {
        return this.branches.get(branchID);
    };

    drawExistedBranches = () => {
        this.branches.forEach(branch => {
            this.drawVertex(branch);
        });
    };

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    };

    resumeAndSyncGraph = () => {
        // add buffered tx
        this.branchToAddAfterResume.forEach(branchID => {
            const b = this.branches.get(branchID);
            if (b) {
                this.drawVertex(b);
            }
        });
        this.branchToAddAfterResume = [];

        // remove removed tx
        this.branchToRemoveAfterResume.forEach(branchID => {
            this.removeVertex(branchID);
        });
        this.branchToRemoveAfterResume = [];
    };

    drawVertex = (branch: branchVertex) => {
        this.vertexChanges++;

        drawBranch(branch, this.graph, this.branches);
    };

    removeVertex = (branchID: string) => {
        this.vertexChanges++;
        this.graph.removeVertex(branchID);
    };

    selectBranch = (branchID: string) => {
        // clear pre-selected branch.
        this.clearSelected(true);

        this.graph.selectVertex(branchID);

        this.updateSelected(branchID);
    };

    centerBranch = (branchID: string) => {
        this.graph.centerVertex(branchID);
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

    start = () => {
        this.graph = new cytoscapeLib([dagre, layoutUtilities], initBranchDAG);

        // add master branch
        const master: branchVertex = {
            ID: '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
            type: 'ConflictBranchType',
            parents: [],
            isConfirmed: true,
            conflicts: null,
            gof: 'GoF(High)',
            aw: 0
        };
        this.branches.set(
            '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
            master
        );
        this.graph.drawVertex({
            data: {
                id: '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
                label: 'master'
            },
            style: {
                'background-color': '#616161',
                label: 'master'
            }
        });
        this.graph.centerVertex(master.ID);

        // set up click event.
        this.graph.cy.on('select', 'node', evt => {
            const node = evt.target;
            const nodeData = node.json();

            this.updateSelected(nodeData.data.id);
        });

        // clear selected node.
        this.graph.cy.on('unselect', 'node', () => {
            this.clearSelected();
        });

        // update layout every 10 seconds if needed.
        this.updateLayoutTimer();
    };

    stop = () => {
        this.unregisterHandlers();

        // stop updating layout.
        clearInterval(this.layoutUpdateTimerID);
    };
}

export default BranchStore;
