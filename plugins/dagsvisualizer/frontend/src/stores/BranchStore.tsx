import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from '../WS';
import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import { dagreOptions } from 'styles/graphStyle';
import layoutUtilities from 'cytoscape-layout-utilities';

export class branchVertex {
	ID:        string;
    type:      string;
	parents:   Array<string>;
	confirmed: boolean;
    conflicts: conflictBranches;
    gof:       string;
    aw:        number;
}

export class conflictBranches {
    branchID:  string;
    conflicts: Array<conflict>;
}

export class conflict {
    outputID:  any;
    branchIDs: Array<string>;
}
export class branchParentUpdate {
    ID:      string;
    parents: Array<string>;
}

export class branchConfirmed {
    ID:             string;
}

export class branchWeightChanged {
    ID: string;
    weight: number;
}

export class BranchStore {
    @observable maxBranchVertices: number = 500;
    @observable branches = new ObservableMap<string, branchVertex>();
    @observable selectedBranch: branchVertex = null;
    @observable paused: boolean = false;
    @observable search: string = "";
    @observable explorerAddress = "localhost:8081";
    branchOrder: Array<any> = [];
    draw: boolean = true;

    vertexChanges = 0;
    branchToRemoveAfterResume = [];
    branchToAddAfterResume = [];

    cy;
    layout;
    layoutApi;
    layoutUpdateTimerID;


    constructor() {
        makeObservable(this);
        registerHandler(WSMsgType.Branch, this.addBranch);
        registerHandler(WSMsgType.BranchParentsUpdate, this.updateParents);
        registerHandler(WSMsgType.BranchConfirmed, this.branchConfirmed);
        registerHandler(WSMsgType.BranchWeightChanged, this.branchWeightChanged);

        cytoscape.use(dagre);
        cytoscape.use(layoutUtilities);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Branch);
        unregisterHandler(WSMsgType.BranchParentsUpdate);
    }

    @action
    addBranch = (branch: branchVertex) => {
        if (this.branchOrder.length >= this.maxBranchVertices) {
            let removed = this.branchOrder.shift();
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
    }

    @action
    updateParents = (newParents: branchParentUpdate) => {
        let b = this.branches.get(newParents.ID);
        if (!b) {
            return;
        }

        b.parents = newParents.parents;
        this.branches.set(newParents.ID, b);
    }

    @action
    branchConfirmed = (confirmedBranch: branchConfirmed) => {
        let b = this.branches.get(confirmedBranch.ID);
        if (!b) {
            return;
        }

        b.confirmed = true;
        this.branches.set(confirmedBranch.ID, b);
    }

    @action
    branchWeightChanged = (branch: branchWeightChanged) => {
        let b = this.branches.get(branch.ID);
        if (!b) {
            return;
        }
        b.aw = branch.weight;
        this.branches.set(branch.ID, b)
    }

    @action
    updateSelected = (branchID: string) => {
      let b = this.branches.get(branchID);
      if (!b) return;
      this.selectedBranch = b;
    }

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
        // unselect preselected node manually
        if (removePreSelectedNode && this.selectedBranch) {
            this.cy.getElementById(this.selectedBranch.ID).unselect();
        }

        this.selectedBranch = null;
    }

    @action
    pauseResume = () => {
        if (this.paused) {
            this.resumeAndSyncGraph();
            this.paused = false;
            return;
        }
        this.paused = true;
    }

    @action
    updateVerticesLimit = (num: number) => {
        this.maxBranchVertices = num;
    }

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    }

    @action
    searchAndHighlight = () => {
        if (!this.search) return;

        this.clearSelected(true);

        let branchNode = this.cy.getElementById(this.search);
        if (!branchNode) return;
        // select the node manually
        branchNode.select();

        this.updateSelected(this.search);
    }

    updateExplorerAddress = (addr: string) => {
        this.explorerAddress = addr;
    }

    drawExistedBranches = () => {
        this.branches.forEach((branch) => {
            this.drawVertex(branch);
        })
    }

    updateDrawStatus = (draw: boolean) => {
        this.draw = draw;
    }

    clearGraph = () => {
        this.cy.elements().remove();
    }

    centerEntireGraph = () => {
        this.cy.center();
    }

    resumeAndSyncGraph = () => {
      // add buffered tx
      this.branchToAddAfterResume.forEach((branchID) => {
        let b = this.branches.get(branchID);
        if (b) {
          this.drawVertex(b);
        }
      })
      this.branchToAddAfterResume = [];

      // remove removed tx
      this.branchToRemoveAfterResume.forEach((branchID) => {
        this.removeVertex(branchID);
      })
      this.branchToRemoveAfterResume = [];
    }

    removeVertex = (branchID: string) => {
        this.vertexChanges++;
        let uiID = '#'+branchID;
        this.cy.remove(uiID);
    }

    drawVertex = (branch: branchVertex) => {
        this.vertexChanges++;

        let v = this.cy.add({
            group: 'nodes',
            data: { id: branch.ID },
        });

        branch.parents.forEach((pID) => {
            let b = this.branches.get(pID);
            if (b) {
                this.cy.add({
                    group: 'edges',
                    data: { source: pID, target: branch.ID}
                });
            }            
        });

        this.layoutApi.placeNewNodes(v);
    }

    updateLayoutTimer = () => {
        this.layoutUpdateTimerID = setInterval(() => {
            if (this.vertexChanges > 0 && !this.paused) {
                this.cy.layout(this.layout).run();
                this.vertexChanges = 0;
            }
        }, 10000);
    }

    start = () => {
        this.cy = cytoscape({
            container: document.getElementById("branchVisualizer"), // container to render in
            style: [ // the stylesheet for the graph
                {
                  selector: 'node',
                  style: {
                    'background-color': '#2E8BC0',
                    'shape': 'rectangle',
                    'width': 25,
                    'height': 15,
                  }
                },            
                {
                  selector: 'edge',
                  style: {
                    'width': 1,
                    'curve-style': 'bezier',
                    'line-color': '#696969',
                    'control-point-step-size': '10px',
                    'events': 'no'
                  }
                },
                {
                  selector: 'node:selected',
                  style: {
                    'background-opacity': 0.333,
                    'background-color': 'red'
                  }
                },
              ],
            layout: {
                name: 'dagre',
            },
        });
        this.layout = dagreOptions;
        this.layoutApi = this.cy.layoutUtilities(
            {
              desiredAspectRatio: 1,
              polyominoGridSizeFactor: 1,
              utilityFunction: 0,
              componentSpacing: 200,
            }
        );

        // add master branch
        let master:branchVertex = {
            ID:             '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
            type:           'ConflictBranchType',
	        parents:        [],
	        confirmed:      true,
            conflicts:      null,
            gof:            "GoF(High)",
            aw:             0,
        }
        this.branches.set("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM", master);
        this.cy.add({
            data: { id: '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM', label: 'master' },
            style: {
                'background-color': '#616161',
                'label': 'master'
            },
            classes: 'top-center'
        });

        // set up click event.
        this.cy.on('select', 'node', (evt) => {
            var node = evt.target;
            const nodeData = node.json();
            
            this.updateSelected(nodeData.data.id);
        });

        // clear selected node.
        this.cy.on('unselect', 'node', (evt) => {
            this.clearSelected();
        });

        // update layout every 10 seconds if needed.
        this.updateLayoutTimer();
    }

    stop = () => {
        this.unregisterHandlers()
        
        // stop updating layout.
        clearInterval(this.layoutUpdateTimerID);
    }
}

export default BranchStore;