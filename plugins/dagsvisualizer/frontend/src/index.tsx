import * as ReactDOM from 'react-dom';
import { Provider } from 'mobx-react';
import TangleStore from 'stores/TangleStore';
import UTXOStore from 'stores/UTXOStore';
import ConflictStore from 'stores/ConflictStore';
import GlobalStore from 'stores/GlobalStore';
import { Root } from 'components/Root';
import React from 'react';

const tangleStore = new TangleStore();
const utxoStore = new UTXOStore();
const conflictStore = new ConflictStore();
const globalStore = new GlobalStore(tangleStore, utxoStore, conflictStore);

const stores = {
    tangleStore: tangleStore,
    utxoStore: utxoStore,
    conflictStore: conflictStore,
    globalStore: globalStore
};

ReactDOM.render(
    <Provider {...stores}>
        <Root />
    </Provider>,
    document.getElementById('root')
);
