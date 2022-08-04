import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import {Transaction} from "app/components/Transaction";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    txId: string
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerTransaction extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getTransaction(this.props.txId);
    }
    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let { txId } = this.props;
        let { query_err, tx } = this.props.explorerStore;
        if (query_err) {
            return (
                <Container>
                    <h4>Transaction not available - 404</h4>
                    <p>
                        Transaction with ID {txId} not found.
                    </p>
                </Container>
            );
        }
        return <Transaction txID={txId} tx={tx}/>
    }
}