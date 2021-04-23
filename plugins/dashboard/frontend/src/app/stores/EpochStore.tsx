import {action, observable} from "mobx";
import {Node as ManaNode} from "app/stores/ManaStore"


enum QueryError {
    NotFound = 1,
    BadRequest = 2
}

class EpochIDResponse {
    epochID: number
}

class EpochData {
    epochID: number;
    epochStartTime: number;
    epochEndTime: number;
    weights: Array<ManaNode>;
    totalWeight: number;
}

export class EpochStore {
    @observable oracleEpoch: EpochData = null;

    // loading
    @observable query_loading: boolean = false;
    @observable query_err: any = null;

    @action getOracleEpoch = async () => {
        try {
            let res = await fetch("/api/epochs/oracle/current");
            if (res.status === 404) {
                this.updateQueryError(QueryError.NotFound);
                return;
            }
            let epochID = (await res.json() as EpochIDResponse).epochID;
            let dataRes = await this.getEpochData(epochID);
            let data = await dataRes as EpochData;
            this.updateOracleEpoch(data);
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
    updateOracleEpoch = (data: EpochData) => {
        data.weights.sort((a, b) => {
            return b.mana - a.mana;
        })
        this.oracleEpoch = data;
    }

    @action
    reset = () => {
        this.oracleEpoch = null;
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