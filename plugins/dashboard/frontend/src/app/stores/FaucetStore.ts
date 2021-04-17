import {action, observable} from 'mobx';
import {RouterStore} from "mobx-react-router";

class SendResult {
    MsgId: string;
}

enum QueryError {
    NotFound
}

export class FaucetStore {
    // send request to faucet
    @observable send_addr: string = "";
    @observable send_access_mana_node_id: string = "";
    @observable send_consensus_mana_node_id: string = "";
    @observable sending: boolean = false;
    @observable sendResult: SendResult = null;
    @observable query_error: string = "";

    routerStore: RouterStore;

    constructor(routerStore: RouterStore) {
        this.routerStore = routerStore;
    }

    sendReq = async () => {
        this.updateSending(true);
        try {
            // send request
            let res = await fetch(`/api/faucet/${this.send_addr}?accessMana=${this.send_access_mana_node_id}&consensusMana=${this.send_consensus_mana_node_id}`);
            if (res.status !== 200) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let result: SendResult = await res.json();
            setTimeout(() => {
                this.updateSendResult(result);
            }, 2000);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    @action
    updateSendResult = (result: SendResult) => {
        this.sending = false;
        this.sendResult = result;
        this.routerStore.history.push(`/explorer/address/${this.send_addr}`);
    };

    @action
    updateSend = (send_addr: string) => {
        this.send_addr = send_addr;
    };

    @action
    updateSendAccessManaNodeID = (access_mana: string) => {
        this.send_access_mana_node_id = access_mana;
    }

    @action
    updateSendConsensusManaNodeID = (consensus_mana: string) => {
        this.send_consensus_mana_node_id = consensus_mana;
    }

    @action
    updateSending = (sending: boolean) => {
        this.sending = sending;
        this.query_error = "";
    };

    @action
    reset = () => {
        this.send_addr = null;
        this.send_access_mana_node_id = "";
        this.send_consensus_mana_node_id = "";
        this.sending = false;
        this.query_error = "";
    };

    @action
    updateQueryError = (err: any) => {
        this.sending = false;
        this.query_error = err;
    };
}

export default FaucetStore;
