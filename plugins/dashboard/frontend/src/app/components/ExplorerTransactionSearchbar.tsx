import * as React from 'react';
import {KeyboardEvent} from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import FormControl from "react-bootstrap/FormControl";
import ExplorerStore from "app/stores/ExplorerStore";
import Row from "react-bootstrap/Row";
import Col from "react-bootstrap/Col";
import InputGroup from "react-bootstrap/InputGroup";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class ExplorerTransactionSearchbar extends React.Component<Props, any> {
    txID: string;

    updateSearch = (e) => {
        this.txID =e.target.value;
    };

    executeSearch = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.explorerStore.routerStore.push(`/explorer/transaction/${this.txID}`);
    };

    render() {
        let {searching} = this.props.explorerStore;

        return (
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <InputGroup className="mb-3">
                            <FormControl
                                placeholder="Transaction ID"
                                aria-label="Transaction ID"
                                aria-describedby="basic-addon1"
                                value={this.txID} onChange={this.updateSearch}
                                onKeyUp={this.executeSearch}
                                disabled={searching}
                            />
                        </InputGroup>
                    </Col>
                </Row>
            </React.Fragment>
        );
    }
}
