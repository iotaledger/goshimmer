import * as React from 'react';
import Container from 'react-bootstrap/Container'
import {inject, observer} from "mobx-react";
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
        this.props.utxoStore.unregisterHandlers();
    }

    render () {
        let { selectedTx } = this.props.utxoStore;

        return (
            <Container>
                <h2> UTXO DAG </h2>
                <div className="graphFrame">
                    <div className="selectedInfo">
                        <p> TXID: </p>
                    </div>
                    <div id="utxoVisualizer" />
                </div>                    
            </Container>
        );
    }
}