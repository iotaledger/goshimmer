import {action, observable} from "mobx";
import {Node as ManaNode} from "app/stores/ManaStore"


enum QueryError {
    NotFound = 1,
    BadRequest = 2
}

class EpochIDResponse {
    epochID: number
}

export class EpochData {
    epochID: number;
    epochStartTime: number;
    epochEndTime: number;
    weights: Array<ManaNode>;
    totalWeight: number;
}

export class EpochStore {
    @observable currentOracleEpoch: EpochData = null;
    @observable previousOracleEpoch: EpochData = null;


    // loading
    @observable query_loading: boolean = false;
    @observable query_err: any = null;

    @action getOracleEpochs = async () => {
        this.updateQueryLoading(true);
        try {
            let res = await fetch("/api/epochs/oracle/current");
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            // get current oracle epoch
            let epochID = (await res.json() as EpochIDResponse).epochID;
            let dataRes = await this.getEpochData(epochID);
            let data = await dataRes as EpochData;
            this.updateOracleEpoch(true, data);

            // get previous oracle epoch
            dataRes = await this.getEpochData(epochID-1);
            data = await dataRes as EpochData;
            this.updateOracleEpoch(false, data);
            this.updateQueryLoading(false);
        } catch (err) {
            this.updateQueryError(err);
        }
    }

    @action
    getEpochData = async (id: number) => {
        try {
            let res = await fetch(`/api/epochs/${id}`)
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            if (res.status === 400) {
                this.updateQueryError(QueryError.BadRequest);
                return;
            }
            return res.json();
        } catch (err) {
            this.updateQueryError(err);
            return;
        }
    }

    @action
    updateOracleEpoch = (updateCurrentOracleEpoch: boolean, data: EpochData) => {
        data.weights.sort((a, b) => {
            return b.mana - a.mana;
        })
        if (updateCurrentOracleEpoch) {
            this.currentOracleEpoch = data;
        } else {
            this.previousOracleEpoch = data;
        }
    }

    @action
    reset = () => {
        this.currentOracleEpoch = null;
        this.previousOracleEpoch = null;
        this.query_err = null;
        this.query_loading = null;
    }

    @action
    updateQueryError = (err: any) => {
        this.query_err = err;
        this.query_loading = false;
    };

    @action
    updateQueryLoading = (loading: boolean) => this.query_loading = loading;
}