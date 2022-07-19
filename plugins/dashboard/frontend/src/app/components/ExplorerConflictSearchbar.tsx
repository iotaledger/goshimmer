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
export class ExplorerConflictSearchbar extends React.Component<Props, any> {
    conflictID: string;

    updateSearch = (e) => {
        this.conflictID =e.target.value;
    };

    executeSearch = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.explorerStore.routerStore.push(`/explorer/conflict/${this.conflictID}`);
    };

    render() {
        let {searching} = this.props.explorerStore;

        return (
            <React.Fragment>
                <Row className={"mb-3"}>
                    <Col>
                        <InputGroup className="mb-3">
                            <FormControl
                                placeholder="Conflict ID"
                                aria-label="Conflict ID"
                                aria-describedby="basic-addon1"
                                value={this.conflictID} onChange={this.updateSearch}
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
