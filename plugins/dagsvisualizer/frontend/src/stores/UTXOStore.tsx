import { action, observable, ObservableMap } from 'mobx';
import {registerHandler, unregisterHandler, WSMsgType} from 'WS';

export class utxoVertex {
    msgID:          string;   
	ID:             string;
	inputs:         Array<string>;
    outputs:        Array<string>;
	approvalWeight: number;
	confirmedTime:  number;
}

export class utxoConfirmed {
    ID: string;
    approvalWeight: number;
    confirmedTime: number;
}


export class UTXOStore {
    @observable maxUTXOVertices: number = 500;
    @observable transactions = new ObservableMap<string, utxoVertex>();
    txOrder: Array<any> = [];

    constructor() {        
        registerHandler(WSMsgType.Transaction, this.addTransaction);
        registerHandler(WSMsgType.TransactionConfirmed, this.setTXConfirmedTime);
    }

    unregisterHandlers() {
        unregisterHandler(WSMsgType.Transaction);
        unregisterHandler(WSMsgType.TransactionConfirmed);
    }

    @action
    addTransaction = (tx: utxoVertex) => {
        if (this.txOrder.length >= this.maxUTXOVertices) {
            let removed = this.txOrder.shift();
            this.transactions.delete(removed);
        }
        console.log(tx.ID)

        this.txOrder.push(tx.ID);
        this.transactions.set(tx.ID, tx);
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
}

export default UTXOStore;