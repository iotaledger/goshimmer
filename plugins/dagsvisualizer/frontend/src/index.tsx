import * as ReactDOM from 'react-dom';
import {Provider} from 'mobx-react';
import TangleStore from 'stores/TangleStore';
import UTXOStore from 'stores/UTXOStore';
import BranchStore from 'stores/BranchStore';
import {Root} from 'components/Root';

const tangleStore = new TangleStore();
const utxoStore = new UTXOStore();
const branchStore = new BranchStore();

const stores = {
  "tangleStore": tangleStore,
  "utxoStore": utxoStore,
  "branchStore": branchStore,
}

ReactDOM.render(
  <Provider {...stores}>
    <Root />
  </Provider>,
  document.getElementById('root')
)