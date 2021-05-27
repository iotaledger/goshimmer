import * as React from 'react';
import {inject, observer} from "mobx-react";
import {ExplorerStore} from "app/stores/ExplorerStore";
import {Transaction} from "app/components/Transaction";

interface Props {
    explorerStore?: ExplorerStore;
}

@inject("explorerStore")
@observer
export class TransactionPayload extends React.Component<Props, any> {
    render() {
        let {payload} = this.props.explorerStore;
        let txID = payload.txID;
        let tx = payload.transaction;
        return <Transaction txID={txID} tx={tx}/>
    }
}
