import { action, observable, ObservableMap } from 'mobx';
import { registerHandler, unregisterHandler, WSMsgType } from 'WS';

export class branchVertex {  
	ID:             string;
    type:           string;
	parents:        Array<string>;
	approvalWeight: number;
	confirmedTime:  number;
}

export class branchParentUpdate {
    ID:      string;
    parents: Array<string>;
}


export class BranchStore {
    @observable maxBranchVertices: number = 500;
    @observable branches = new ObservableMap<string, branchVertex>();
    branchOrder: Array<any> = [];

    constructor() {        
        registerHandler(WSMsgType.Branch, this.addBranch);
        registerHandler(WSMsgType.BranchParentsUpdate, this.updateParents);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Transaction);
        unregisterHandler(WSMsgType.TransactionConfirmed);
    }

    @action
    addBranch = (branch: branchVertex) => {
        if (this.branchOrder.length >= this.maxBranchVertices) {
            let removed = this.branchOrder.shift();
            this.branches.delete(removed);
        }
        console.log(branch.ID, branch.type);

        this.branchOrder.push(branch.ID);
        this.branches.set(branch.ID, branch);
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
}

export default BranchStore;