import * as ReactDOM from 'react-dom';
import { Provider } from 'mobx-react';
import TangleStore from 'stores/TangleStore';
import UTXOStore from 'stores/UTXOStore';
import BranchStore from 'stores/BranchStore';
import GlobalStore from 'stores/GlobalStore';
import { Root } from 'components/Root';
import React from 'react';

const tangleStore = new TangleStore();
const utxoStore = new UTXOStore();
const branchStore = new BranchStore();
const globalStore = new GlobalStore(tangleStore, utxoStore, branchStore);

const stores = {
    tangleStore: tangleStore,
    utxoStore: utxoStore,
    branchStore: branchStore,
    globalStore: globalStore
};

ReactDOM.render(
    <Provider {...stores}>
        <Root />
    </Provider>,
    document.getElementById('root')
);
