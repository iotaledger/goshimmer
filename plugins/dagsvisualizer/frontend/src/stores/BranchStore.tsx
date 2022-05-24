import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from 'utils/WS';
import { MAX_VERTICES } from 'utils/constants';
import dagre from 'cytoscape-dagre';
import layoutUtilities from 'cytoscape-layout-utilities';
import {
    cytoscapeLib,
    drawBranch,
    initBranchDAG,
    removeConfirmationStyle,
    updateConfirmedBranch
} from 'graph/cytoscape';
import {
    branchGoFChanged,
    branchParentUpdate,
    branchVertex,
    branchWeightChanged
} from 'models/branch';
import { BRANCH } from '../styles/cytoscapeStyles';

export class BranchStore {
    @observable maxBranchVertices = MAX_VERTICES;
    @observable branches = new ObservableMap<string, branchVertex>();
    @observable foundBranches = new ObservableMap<string, branchVertex>();
    branchesBeforeSearching: Map<string, branchVertex>;
    @observable selectedBranch: branchVertex = null;
    @observable paused = false;
    @observable search = '';
    branchOrder: Array<any> = [];
    highlightedBranches = [];
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
        registerHandler(WSMsgType.BranchGoFChanged, this.branchGoFChanged);
        registerHandler(
            WSMsgType.BranchWeightChanged,
            this.branchWeightChanged
        );
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Branch);
        unregisterHandler(WSMsgType.BranchParentsUpdate);
        unregisterHandler(WSMsgType.BranchGoFChanged);
        unregisterHandler(WSMsgType.BranchWeightChanged);
    }

    @action
    addBranch = (branch: branchVertex) => {
        this.checkLimit();
        this.branchOrder.push(branch.ID);

        if (this.paused) {
            this.branchToAddAfterResume.push(branch.ID);
        }
        if (this.draw) {
            this.drawVertex(branch);
        }
    };

    checkLimit = () => {
        if (this.branchOrder.length >= this.maxBranchVertices) {
            const removed = this.branchOrder.shift();
            if (this.paused) {
                // keep the removed tx that should be removed from the graph after resume.
                this.branchToRemoveAfterResume.push(removed);
            } else {
                this.removeVertex(removed);
            }
        }
    };

    @action
    addFoundBranch = (branch: branchVertex) => {
        this.foundBranches.set(branch.ID, branch);
    };

    @action
    clearFoundBranches = () => {
        this.foundBranches.clear();
    };

    @action
    updateParents = (newParents: branchParentUpdate) => {
        const b = this.branches.get(newParents.ID);
        if (!b) {
            return;
        }

        b.parents = newParents.parents;
        // draw new links
        this.drawVertex(b);
    };

    @action
    branchGoFChanged = (branch: branchGoFChanged) => {
        const b = this.branches.get(branch.ID);
        if (!b) {
            return;
        }

        if (branch.isConfirmed) {
            b.isConfirmed = true;
        } else {
            b.isConfirmed = false;
        }

        b.gof = branch.gof;
        this.branches.set(branch.ID, b);
        updateConfirmedBranch(b, this.graph);
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
        const b =
            this.branches.get(branchID) || this.foundBranches.get(branchID);
        if (!b) return;
        this.selectedBranch = b;
        removeConfirmationStyle(b.ID, this.graph);
    };

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
        // unselect preselected node manually
        if (removePreSelectedNode && this.selectedBranch) {
            this.graph.unselectVertex(this.selectedBranch.ID);
        }
        updateConfirmedBranch(this.selectedBranch, this.graph);
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
        this.trimBranchToVerticesLimit();
    };

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    };

    @action
    searchAndHighlight = () => {
        if (!this.search) return;

        this.selectBranch(this.search);
        this.centerBranch(this.search);
    };

    getBranchVertex = (branchID: string) => {
        return this.branches.get(branchID) || this.foundBranches.get(branchID);
    };

    drawExistedBranches = () => {
        for (const branch of this.branchesBeforeSearching.values()) {
            this.drawVertex(branch);
        }
        this.resumeAndSyncGraph();
        this.branchesBeforeSearching = undefined;
    };

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    };

    resumeAndSyncGraph = () => {
        // add buffered tx
        this.branchToAddAfterResume.forEach((branchID) => {
            const b = this.branches.get(branchID);
            if (b) {
                this.drawVertex(b);
            }
        });
        this.branchToAddAfterResume = [];

        // remove removed tx
        this.branchToRemoveAfterResume.forEach((branchID) => {
            this.removeVertex(branchID);
        });
        this.branchToRemoveAfterResume = [];
    };

    drawVertex = async (branch: branchVertex) => {
        this.vertexChanges++;
        await drawBranch(branch, this.graph, this.branches);
        updateConfirmedBranch(branch, this.graph);
    };

    removeVertex = (branchID: string) => {
        this.vertexChanges++;
        this.graph.removeVertex(branchID);
        this.branches.delete(branchID);
    };

    highlightBranches = (branchIDs: string[]) => {
        this.clearHighlightedBranches();

        // update highlighted branches
        this.highlightedBranches = branchIDs;
        branchIDs.forEach((id) => {
            this.graph.selectVertex(id);
        });
    };

    clearHighlightedBranches = () => {
        this.highlightedBranches.forEach((id) => {
            this.graph.unselectVertex(id);
        });
    };

    selectBranch = (branchID: string) => {
        // clear pre-selected branch.
        this.clearSelected(true);
        this.graph.selectVertex(branchID);
        this.updateSelected(branchID);
        removeConfirmationStyle(this.selectedBranch.ID, this.graph);
    };

    centerBranch = (branchID: string) => {
        this.graph.centerVertex(branchID);
    };

    centerEntireGraph = () => {
        this.graph.centerGraph();
    };

    clearGraph = () => {
        this.graph.clearGraph();
        if (!this.branchesBeforeSearching) {
            this.branchesBeforeSearching = new Map<string, branchVertex>();
            this.branches.forEach((branch, branchID) => {
                this.branchesBeforeSearching.set(branchID, branch);
            });
        }
        this.branches.clear();
        this.addMasterBranch();
    };

    updateLayoutTimer = () => {
        this.layoutUpdateTimerID = setInterval(() => {
            if (this.vertexChanges > 0 && !this.paused) {
                this.graph.updateLayout();
                this.vertexChanges = 0;
            }
        }, 10000);
    };

    trimBranchToVerticesLimit() {
        if (this.branchOrder.length >= this.maxBranchVertices) {
            const removeStartIndex =
                this.branchOrder.length - this.maxBranchVertices;
            const removed = this.branchOrder.slice(0, removeStartIndex);
            this.branchOrder = this.branchOrder.slice(removeStartIndex);
            this.removeBranches(removed);
        }
    }

    removeBranches(removed: string[]) {
        removed.forEach((id: string) => {
            const b = this.branches.get(id);
            if (b) {
                this.removeVertex(id);
                this.branches.delete(id);
            }
        });
    }

    addMasterBranch = (): branchVertex => {
        const master: branchVertex = {
            ID: '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
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
                'background-color': BRANCH.MASTER_COLOR,
                label: 'master',
                color: BRANCH.MASTER_LABEL
            }
        });
        return master;
    };

    start = () => {
        this.graph = new cytoscapeLib([dagre, layoutUtilities], initBranchDAG);

        // add master branch
        const master = this.addMasterBranch();
        this.graph.centerVertex(master.ID);

        // set up click event.
        this.graph.addNodeEventListener('select', (evt) => {
            const node = evt.target;
            const nodeData = node.json();

            this.updateSelected(nodeData.data.id);
        });

        // clear selected node.
        this.graph.addNodeEventListener('unselect', () => {
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
