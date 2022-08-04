import {action, makeObservable, observable, ObservableMap} from 'mobx';
import {registerHandler, unregisterHandler, WSBlkType} from 'utils/WS';
import {MAX_VERTICES} from 'utils/constants';
import dagre from 'cytoscape-dagre';
import layoutUtilities from 'cytoscape-layout-utilities';
import {
    cytoscapeLib,
    drawConflict,
    initConflictDAG,
    removeConfirmationStyle,
    updateConfirmedConflict
} from 'graph/cytoscape';
import {
    conflictConfirmationStateChanged,
    conflictParentUpdate,
    conflictVertex,
    conflictWeightChanged
} from 'models/conflict';
import { BRANCH } from '../styles/cytoscapeStyles';

export class ConflictStore {
    @observable maxConflictVertices = MAX_VERTICES;
    @observable conflicts = new ObservableMap<string, conflictVertex>();
    @observable foundConflicts = new ObservableMap<string, conflictVertex>();
    conflictsBeforeSearching: Map<string, conflictVertex>;
    @observable selectedConflict: conflictVertex = null;
    @observable paused = false;
    @observable search = '';
    conflictOrder: Array<any> = [];
    highlightedConflicts = [];
    draw = true;

    vertexChanges = 0;
    conflictToRemoveAfterResume = [];
    conflictToAddAfterResume = [];

    layoutUpdateTimerID;
    graph;

    constructor() {
        makeObservable(this);
        registerHandler(WSBlkType.Conflict, this.addConflict);
        registerHandler(WSBlkType.ConflictParentsUpdate, this.updateParents);
        registerHandler(WSBlkType.ConflictConfirmationStateChanged, this.conflictConfirmationStateChanged);
        registerHandler(
            WSBlkType.ConflictWeightChanged,
            this.conflictWeightChanged
        );
    }

    unregisterHandlers() {
        unregisterHandler(WSBlkType.Conflict);
        unregisterHandler(WSBlkType.ConflictParentsUpdate);
        unregisterHandler(WSBlkType.ConflictConfirmationStateChanged);
        unregisterHandler(WSBlkType.ConflictWeightChanged);
    }

    @action
    addConflict = (conflict: conflictVertex) => {
        this.checkLimit();
        this.conflictOrder.push(conflict.ID);

        if (this.paused) {
            this.conflictToAddAfterResume.push(conflict.ID);
        }
        if (this.draw) {
            this.drawVertex(conflict);
        }
    };

    checkLimit = () => {
        if (this.conflictOrder.length >= this.maxConflictVertices) {
            const removed = this.conflictOrder.shift();
            if (this.paused) {
                // keep the removed tx that should be removed from the graph after resume.
                this.conflictToRemoveAfterResume.push(removed);
            } else {
                this.removeVertex(removed);
            }
        }
    };

    @action
    addFoundConflict = (conflict: conflictVertex) => {
        this.foundConflicts.set(conflict.ID, conflict);
    };

    @action
    clearFoundConflicts = () => {
        this.foundConflicts.clear();
    };

    @action
    updateParents = (newParents: conflictParentUpdate) => {
        const b = this.conflicts.get(newParents.ID);
        if (!b) {
            return;
        }

        b.parents = newParents.parents;
        // draw new links
        this.drawVertex(b);
    };

    @action
    conflictConfirmationStateChanged = (conflict: conflictConfirmationStateChanged) => {
        const b = this.conflicts.get(conflict.ID);
        if (!b) {
            return;
        }

        if (conflict.isConfirmed) {
            b.isConfirmed = true;
        } else {
            b.isConfirmed = false;
        }

        b.confirmationState = conflict.confirmationState;
        this.conflicts.set(conflict.ID, b);
        updateConfirmedConflict(b, this.graph);
    };

    @action
    conflictWeightChanged = (conflict: conflictWeightChanged) => {
        const b = this.conflicts.get(conflict.ID);
        if (!b) {
            return;
        }
        b.aw = conflict.weight;
        b.confirmationState = conflict.confirmationState;
        this.conflicts.set(conflict.ID, b);
    };

    @action
    updateSelected = (conflictID: string) => {
        const b =
            this.conflicts.get(conflictID) || this.foundConflicts.get(conflictID);
        if (!b) return;
        this.selectedConflict = b;
        removeConfirmationStyle(b.ID, this.graph);
    };

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
        // unselect preselected node manually
        if (removePreSelectedNode && this.selectedConflict) {
            this.graph.unselectVertex(this.selectedConflict.ID);
        }
        updateConfirmedConflict(this.selectedConflict, this.graph);
        this.selectedConflict = null;
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
        this.maxConflictVertices = num;
        this.trimConflictToVerticesLimit();
    };

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    };

    @action
    searchAndHighlight = () => {
        if (!this.search) return;

        this.selectConflict(this.search);
        this.centerConflict(this.search);
    };

    getConflictVertex = (conflictID: string) => {
        return this.conflicts.get(conflictID) || this.foundConflicts.get(conflictID);
    };

    drawExistedConflicts = () => {
        for (const conflict of this.conflictsBeforeSearching.values()) {
            this.drawVertex(conflict);
        }
        this.resumeAndSyncGraph();
        this.conflictsBeforeSearching = undefined;
    };

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    };

    resumeAndSyncGraph = () => {
        // add buffered tx
        this.conflictToAddAfterResume.forEach((conflictID) => {
            const b = this.conflicts.get(conflictID);
            if (b) {
                this.drawVertex(b);
            }
        });
        this.conflictToAddAfterResume = [];

        // remove removed tx
        this.conflictToRemoveAfterResume.forEach((conflictID) => {
            this.removeVertex(conflictID);
        });
        this.conflictToRemoveAfterResume = [];
    };

    drawVertex = async (conflict: conflictVertex) => {
        this.vertexChanges++;
        await drawConflict(conflict, this.graph, this.conflicts);
        updateConfirmedConflict(conflict, this.graph);
    };

    removeVertex = (conflictID: string) => {
        this.vertexChanges++;
        this.graph.removeVertex(conflictID);
        this.conflicts.delete(conflictID);
    };

    highlightConflicts = (conflictIDs: string[]) => {
        this.clearHighlightedConflicts();

        // update highlighted conflicts
        this.highlightedConflicts = conflictIDs;
        conflictIDs.forEach((id) => {
            this.graph.selectVertex(id);
        });
    };

    clearHighlightedConflicts = () => {
        this.highlightedConflicts.forEach((id) => {
            this.graph.unselectVertex(id);
        });
    };

    selectConflict = (conflictID: string) => {
        // clear pre-selected conflict.
        this.clearSelected(true);
        this.graph.selectVertex(conflictID);
        this.updateSelected(conflictID);
        removeConfirmationStyle(this.selectedConflict.ID, this.graph);
    };

    centerConflict = (conflictID: string) => {
        this.graph.centerVertex(conflictID);
    };

    centerEntireGraph = () => {
        this.graph.centerGraph();
    };

    clearGraph = () => {
        this.graph.clearGraph();
        if (!this.conflictsBeforeSearching) {
            this.conflictsBeforeSearching = new Map<string, conflictVertex>();
            this.conflicts.forEach((conflict, conflictID) => {
                this.conflictsBeforeSearching.set(conflictID, conflict);
            });
        }
        this.conflicts.clear();
        this.addMasterConflict();
    };

    updateLayoutTimer = () => {
        this.layoutUpdateTimerID = setInterval(() => {
            if (this.vertexChanges > 0 && !this.paused) {
                this.graph.updateLayout();
                this.vertexChanges = 0;
            }
        }, 10000);
    };

    trimConflictToVerticesLimit() {
        if (this.conflictOrder.length >= this.maxConflictVertices) {
            const removeStartIndex =
                this.conflictOrder.length - this.maxConflictVertices;
            const removed = this.conflictOrder.slice(0, removeStartIndex);
            this.conflictOrder = this.conflictOrder.slice(removeStartIndex);
            this.removeConflicts(removed);
        }
    }

    removeConflicts(removed: string[]) {
        removed.forEach((id: string) => {
            const b = this.conflicts.get(id);
            if (b) {
                this.removeVertex(id);
                this.conflicts.delete(id);
            }
        });
    }

    addMasterConflict = (): conflictVertex => {
        const master: conflictVertex = {
            ID: '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
            parents: [],
            isConfirmed: true,
            conflicts: null,
            confirmationState: 'Confirmed',
            aw: 0
        };
        this.conflicts.set(
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
        this.graph = new cytoscapeLib([dagre, layoutUtilities], initConflictDAG);

        // add master conflict
        const master = this.addMasterConflict();
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

export default ConflictStore;
