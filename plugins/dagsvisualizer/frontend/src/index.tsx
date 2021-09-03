import * as React from 'react';
import * as ReactDOM from 'react-dom';
import {Provider} from 'mobx-react';
import Todo from 'stores/Todo';

const todoStore = new Todo();

ReactDOM.render(
  <Provider todo={todoStore}>
    <div>
      <h1>Hello</h1>
    </div>
  </Provider>,
  document.getElementById('root')
)