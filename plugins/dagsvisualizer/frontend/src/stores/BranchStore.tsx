import { action, observable, ObservableMap } from 'mobx';
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
    conflictIDs:    Array<string>;
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
    @observable selectedBranch: branchVertex;
    branchOrder: Array<any> = [];
    newVertexCounter = 0;
    cy;
    layout;
    layoutApi;


    constructor() {        
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
        console.log(branch.ID, branch.type);

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

    removeVertex = (branchID: string) => {
        let uiID = '#'+branchID;
        this.cy.remove(uiID);
        this.cy.layout( dagreOptions ).run();
    }

    drawVertex = (branch: branchVertex) => {
        this.newVertexCounter++;

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
        this.cy.layout(dagreOptions).run();
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
                    'control-point-step-size': '10px'
                  }
                }
              ],
            layout: {
                name: 'fcose',
            },
        });
        this.layoutApi = this.cy.layoutUtilities(
            {
              desiredAspectRatio: 1,
              polyominoGridSizeFactor: 1,
              utilityFunction: 0,
              componentSpacing: 200,
            }
        );

        // add master branch
        let master = {
            ID:             '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM',
            type:           'ConflictBranchType',
	        parents:        [],
	        approvalWeight: 1.0,
	        confirmedTime:  123,
            conflictIDs:    []
        }
        this.branches.set("4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM", master);
        this.cy.add({
            data: { id: '4uQeVj5tqViQh7yWWGStvkEG1Zmhx6uasJtWCJziofM', label: 'master' },
            style: { // style property overrides 
                'background-color': '#616161',
                'label': 'master'
            },
            classes: 'top-center'
        });
    }
}

export default BranchStore;