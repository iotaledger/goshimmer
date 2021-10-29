import { action, makeObservable, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from 'WS';
import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import { dagreOptions } from 'styles/graphStyle';
import layoutUtilities from 'cytoscape-layout-utilities';

export class branchVertex {  
	ID:             string;
    type:           string;
	parents:        Array<string>;
	approvalWeight: number;
	confirmedTime:  number;
    conflicts:      conflictBranches;
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

export class branchAWUpdate {
    ID:             string;
    conflicts:      Array<string>;
    approvalWeight: number;
}

export class BranchStore {
    @observable maxBranchVertices: number = 500;
    @observable branches = new ObservableMap<string, branchVertex>();
    @observable selectedBranch: branchVertex = null;
    branchOrder: Array<any> = [];
    vertexChanges = 0;
    cy;
    layout;
    layoutApi;
    layoutUpdateTimerID;


    constructor() {      
        makeObservable(this);  
        registerHandler(WSMsgType.Branch, this.addBranch);
        registerHandler(WSMsgType.BranchParentsUpdate, this.updateParents);
        registerHandler(WSMsgType.BranchAWUpdate, this.updateAW);

        cytoscape.use(dagre);
        cytoscape.use(layoutUtilities);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Branch);
        unregisterHandler(WSMsgType.BranchParentsUpdate);
        unregisterHandler(WSMsgType.BranchAWUpdate);
    }

    @action
    addBranch = (branch: branchVertex) => {
        if (this.branchOrder.length >= this.maxBranchVertices) {
            let removed = this.branchOrder.shift();
            this.branches.delete(removed);
            this.removeVertex(removed.ID);
        }
        console.log(branch.conflicts);

        this.branchOrder.push(branch.ID);
        this.branches.set(branch.ID, branch);

        this.drawVertex(branch);
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
    updateAW = (newAW: branchAWUpdate) => {
        let b = this.branches.get(newAW.ID);
        if (!b) {
            return;
        }

        b.approvalWeight = newAW.approvalWeight;
        this.branches.set(newAW.ID, b);

        // update AW of conflict branches
        newAW.conflicts.forEach((id) => {
            let b = this.branches.get(id);
            if (b) {    
                b.approvalWeight = newAW.approvalWeight;
                this.branches.set(id, b);
            }
        })
    }

    @action
    updateSelected = (branchID: string) => {
      let b = this.branches.get(branchID);
      this.selectedBranch = b;
    }

    @action
    clearSelected = () => {
        this.selectedBranch = null;
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
            console.log(pID);
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
            if (this.vertexChanges > 0) {
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
	        approvalWeight: 1.0,
	        confirmedTime:  123,
            conflicts:      null
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