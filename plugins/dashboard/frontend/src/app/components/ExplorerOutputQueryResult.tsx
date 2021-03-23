import * as React from 'react';
import Container from "react-bootstrap/Container";
import ListGroup from "react-bootstrap/ListGroup";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import Badge from "react-bootstrap/Badge";
import {displayManaUnit} from "app/utils";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
    match?: {
        params: {
            id: string,
        }
    }
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerOutputQueryResult extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getOutput(this.props.match.params.id);
        this.props.explorerStore.getPendingMana(this.props.match.params.id);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let {id} = this.props.match.params;
        let { query_err, output, pendingMana } = this.props.explorerStore;

        if (query_err) {
            return (
                <Container>
                    <h4>Output not found - 404</h4>
                    <span>{id}</span>
                </Container>
            );
        }
        return (
            <Container>
                <h4>Output</h4>
                {output && <div>
                    <ListGroup>
                        <ListGroup.Item>Output ID: {output.outputID.base58}</ListGroup.Item>
                        <ListGroup.Item>Transaction ID: <a href={`/explorer/transaction/${output.outputID.transactionID}`}>{output.outputID.transactionID}</a> </ListGroup.Item>
                        <ListGroup.Item>Index: {output.outputID.outputIndex}</ListGroup.Item>
                        <ListGroup.Item>Type: {output.type}</ListGroup.Item>
                        <ListGroup.Item>
                            Balances:
                            <span>
                                {Object.entries(output.balances).map((entry, i) => (<div className={"mr-2"} key={i}><Badge variant="secondary">{entry[1]} {entry[0]}</Badge></div>))}
                            </span>
                        </ListGroup.Item>
                        {pendingMana && <ListGroup.Item>
                            Pending Mana
                            <hr/>
                            <div>Value: {displayManaUnit(pendingMana.mana)}</div>
                            <div>Timestamp: {new Date(pendingMana.timestamp * 1000).toLocaleString()}</div>
                        </ListGroup.Item>}
                    </ListGroup>
                </div>}
            </Container>
        )
    }
}