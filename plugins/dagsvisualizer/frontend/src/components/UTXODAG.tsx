import * as React from 'react';
import Container from 'react-bootstrap/Container'
import {inject, observer} from "mobx-react";
import UTXOStore from "stores/UTXOStore";

interface Props {
    utxoStore?: UTXOStore;
}

@inject("utxoStore")
@observer
export class UTXODAG extends React.Component {
    render () {
        return (
            <Container>
                <h2> UTXO DAG </h2>
                <div id="utxoVisualizer" />
            </Container>
        );
    }
}