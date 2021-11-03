import { action, makeObservable, observable, ObservableMap } from 'mobx';
import {registerHandler, unregisterHandler, WSMsgType} from 'WS';
import cytoscape from 'cytoscape';
import dagre from 'cytoscape-dagre';
import { dagreOptions } from 'styles/graphStyle';
import layoutUtilities from 'cytoscape-layout-utilities';
import 'styles/style.css';

export class utxoVertex {
  msgID:          string;   
	ID:             string;
	inputs:         Array<input>;
  outputs:        Array<string>;
	approvalWeight: number;
	confirmedTime:  number;
}

export class input {
    type:               string;
    referencedOutputID: any;
}
export class utxoConfirmed {
    ID: string;
    approvalWeight: number;
    confirmedTime: number;
}


export class UTXOStore {
    @observable maxUTXOVertices: number = 500;
    @observable transactions = new ObservableMap<string, utxoVertex>();
    @observable selectedTx: utxoVertex = null;
    @observable paused: boolean = false;
    @observable search: string = "";
    outputMap = new Map();
    txOrder: Array<any> = [];

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
        registerHandler(WSMsgType.TransactionConfirmed, this.setTXConfirmedTime);

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
            let removed = this.txOrder.shift();
            let txObj = this.transactions.get(removed);
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

        this.txOrder.push(tx.ID);
        this.transactions.set(tx.ID, tx);
        tx.outputs.forEach((outputID) => {
          this.outputMap.set(outputID, {});
        })

        if (this.paused) {
          this.txToAddAfterResume.push(tx.ID);
        } else {
          this.drawVertex(tx);
        }
    }

    @action
    setTXConfirmedTime = (txConfirmed: utxoConfirmed) => {
        let tx = this.transactions.get(txConfirmed.ID);
        if (!tx) {
            return;
        }

        tx.confirmedTime = txConfirmed.confirmedTime;
        tx.approvalWeight = txConfirmed.approvalWeight;
        this.transactions.set(txConfirmed.ID, tx);
    }

    @action
    updateSelected = (txID: string) => {
      let tx = this.transactions.get(txID);
      if (!tx) return;
      this.selectedTx = tx;
    }

    @action
    clearSelected = (removePreSelectedNode?: boolean) => {
      // unselect preselected node manually
      if (removePreSelectedNode && this.selectedTx) {
          this.cy.getElementById(this.selectedTx.ID).unselect();
      }

      this.selectedTx = null;
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
        this.maxUTXOVertices = num;
    }

    @action
    updateSearch = (search: string) => {
        this.search = search.trim();
    }

    @action
    searchAndHighlight = () => {
        if (!this.search) return;

        this.clearSelected(true);
        
        let txNode = this.cy.getElementById(this.search);
        if (!txNode) return;
        // select the node manually
        txNode.select();
        
        this.updateSelected(this.search);
    }

    resumeAndSyncGraph = () => {
      // add buffered tx
      this.txToAddAfterResume.forEach((txID) => {
        let tx = this.transactions.get(txID);
        if (tx) {
          this.drawVertex(tx);
        }        
      })

      // remove removed tx
      this.txToRemoveAfterResume.forEach((txID) => {
        this.removeVertex(txID);
      })
      this.txToRemoveAfterResume = [];
    }

    removeVertex = (txID: string) => {
        let children = this.cy.getElementById(txID).children();

        this.cy.remove('#'+txID);
        this.cy.remove(children);
        this.vertexChanges++;
    }

    drawVertex = (tx: utxoVertex) => {
        this.vertexChanges++;
        let collection = this.cy.collection();       

        // draw grouping (tx)
        collection = collection.union(this.cy.add({
          group: 'nodes',
          data: { id: tx.ID },
          classes: 'transaction'
        }));

        // draw inputs
        let inputIDs = [];
        let i = 0;
        tx.inputs.forEach((input) => {
            // input node
            let ID = hashString(input.referencedOutputID.base58+tx.ID+'_input');
            collection = collection.union(this.cy.add(
                {
                  group: 'nodes',
                  data: { id: ID, parent: tx.ID, input: input.referencedOutputID.base58 },
                  classes: 'input'
                }
              ));
            
            // input alignment edges
            if (i > 0) {
              collection = collection.union(this.cy.add({
                  group: "edges",
                  data: { source: inputIDs[i], target: ID },
                  classes: 'invisible'
                }));
            }
            inputIDs.push(ID);
            i++;

            // link input to the unspent output
            let spentOutput = this.outputMap.get(input.referencedOutputID.base58);
            if (spentOutput) {
                collection = collection.union(this.cy.add(
                  {
                    group: 'edges',
                    data: { source: input.referencedOutputID.base58, target: ID}
                  }
                ));
            }
        });

        // draw outputs
        let outputIDs = []; i = 0;
        tx.outputs.forEach((outputID) => {
            collection = collection.union(this.cy.add({
                group: "nodes",
                data: { id: outputID, parent: tx.ID },
                classes: 'output'
            }));

            // edges for alignment
            if (i > 0) {
              collection = collection.union(this.cy.add({
                  group: "edges",
                  data: { source: outputIDs[i], target: outputID },
                  classes: 'invisible'
                }));
            }
            outputIDs.push(outputID);
            i++;
        })
        
        // alignment of inputs and outputs
        let inIndex = Math.floor(inputIDs.length/2);
        let outIndex = Math.floor(outputIDs.length/2);
        collection = collection.union(this.cy.add({
          group: "edges",
          data: { source: inputIDs[inIndex], target: outputIDs[outIndex] },
          classes: 'invisible'
        }));

        this.layoutApi.placeNewNodes(collection);
        this.cy.layout(this.layout).run();
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
            container: document.getElementById("utxoVisualizer"), // container to render in
            style: [ // the stylesheet for the graph
                {
                  selector: 'node',
                  style: {
                    'font-weight': 'bold',
                    'shape': 'rectangle',
                    'width': 20,
                    'height': 20,
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
                      'events': 'no'
                    }
                },
                {
                  selector: '.output',
                    style: {
                      'background-color': '#FBE698',
                      'events': 'no'
                    }
                },
                {
                  selector: '.invisible',
                    style: {
                      'visibility': 'hidden'
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
              componentSpacing: 80,
            }
        );

        // set up click event
        this.cy.on('select', 'node', (evt) => {
          var node = evt.target;
          const nodeData = node.json();
          
          this.updateSelected(nodeData.data.id);
        });

        // clear selected node
        this.cy.on('unselect', 'node', (evt) => {
          this.clearSelected();
        });

        this.updateLayoutTimer();
    }

    stop = () => {
      this.unregisterHandlers()
      
      // stop updating layout.
      clearInterval(this.layoutUpdateTimerID);
      // maybe store graph history?
  }
}

export default UTXOStore;

function hashString(source: string) {
    var hash = 0;
    if (source.length === 0) {
        return hash;
    }
    for (var i = 0; i < source.length; i++) {
        var char = source.charCodeAt(i);
        hash = ((hash<<5)-hash)+char;
        hash = hash & hash; // Convert to 32bit integer
    }
    return hash.toString();
}