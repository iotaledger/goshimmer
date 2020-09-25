import {action, computed, observable, ObservableMap} from 'mobx';
import {registerHandler, WSMsgType} from "app/misc/WS";

class ManaMsg {
    access: number;
    consensus: number;
    // in ms?
    time: string;
}

const maxRegistrySize = 100;

export class ManaStore {
    // mana values
    @observable manaValues: Array<any> = [];


    constructor() {
        registerHandler(WSMsgType.Mana, this.addNewManaValue);
    };

    @action
    addNewManaValue(manaMsg: ManaMsg) {
        if (this.manaValues.length == maxRegistrySize) {
            // shift if we already have enough values
            this.manaValues.shift()
        }
        let newManaData = [new Date(manaMsg.time), manaMsg.access, manaMsg.consensus]
        this.manaValues.push(newManaData)
    }
}