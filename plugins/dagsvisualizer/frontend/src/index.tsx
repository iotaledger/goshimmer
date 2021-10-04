import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Provider} from 'mobx-react';
import TangleStore from 'stores/TangleStore';
import UTXOStore from 'stores/UTXOStore';

const tangleStore = new TangleStore();
const utxoStore = new UTXOStore();

const stores = {
  "tangleStore": tangleStore,
  "utxoStore": utxoStore,
}

ReactDOM.render(
  <Provider {...stores}>
    <div>
      <h1>Hello</h1>
    </div>
  </Provider>,
  document.getElementById('root')
)