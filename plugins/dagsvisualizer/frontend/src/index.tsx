import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Provider} from 'mobx-react';
import Container from 'react-bootstrap/Container';
import Row from "react-bootstrap/Row";
import TangleStore from 'stores/TangleStore';
import UTXOStore from 'stores/UTXOStore';
import BranchStore from 'stores/BranchStore';
import {TangleDAG} from 'components/TangleDAG';
import {UTXODAG} from 'components/UTXODAG';
import {BranchDAG} from 'components/BranchDAG';

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
    <Container>
      <Row>
        <TangleDAG />
      </Row>
      <Row>
        <UTXODAG />
      </Row>
      <Row>
        <BranchDAG />
      </Row>
    </Container>
  </Provider>,
  document.getElementById('root')
)