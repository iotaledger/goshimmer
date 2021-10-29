import * as React from 'react';
import Container from 'react-bootstrap/Container';
import {inject, observer} from "mobx-react";
import {TransactionInfo} from "components/TransactionInfo";
import UTXOStore from "stores/UTXOStore";

interface Props {
    utxoStore?: UTXOStore;
}

@inject("utxoStore")
@observer
export class UTXODAG extends React.Component<Props, any> {
    componentDidMount() {
        this.props.utxoStore.start();
    }

    componentWillUnmount() {
        this.props.utxoStore.stop();
    }

    render () {
        return (
            <Container>
                <h2> UTXO DAG </h2>
                <div className="graphFrame">
                    <TransactionInfo />
                    <div id="utxoVisualizer" />
                </div>                    
            </Container>
        );
    }
}