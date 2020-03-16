import {action, observable} from 'mobx';

class SendResult {
    Resp: string;
}

enum QueryError {
    NotFound
}

export class FaucetStore {
    // send request to faucet
    @observable send_addr: string = "";
    @observable sending: boolean = false;
    @observable sendResult: SendResult = null;

    constructor() {
    }

    sendReq = async () => {
        this.updateSending(true);
        try {
            // send request
            let res = await fetch(`/api/faucet/${this.send_addr}`);
            if (res.status !== 200) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let result: SendResult = await res.json();
            this.updateSendResult(result);
        } catch (err) {
            this.updateQueryError(err);
        }
    };

    @action
    updateSendResult = (result: SendResult) => {
        this.sending = false;
        this.sendResult = result;
    };

    @action
    resetSend = () => {
        this.sending = false;
    };

    @action
    updateSend = (send_addr: string) => {
        this.send_addr = send_addr;
    };

    @action
    updateSending = (sending: boolean) => this.sending = sending;

    @action
    reset = () => {
        this.send_addr = null;
    };

    @action
    updateQueryError = (err: any) => {
        this.sending = false;
    };
}

export default FaucetStore;
