import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Provider} from 'mobx-react';
import TangleStore from 'stores/TangleStore';

const tangleStore = new TangleStore();

ReactDOM.render(
  <Provider tanglestore={tangleStore}>
    <div>
      <h1>Hello</h1>
    </div>
  </Provider>,
  document.getElementById('root')
)