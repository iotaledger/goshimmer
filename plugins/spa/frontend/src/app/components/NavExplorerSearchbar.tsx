import * as React from 'react';
import {KeyboardEvent} from 'react';
import NodeStore from "app/stores/NodeStore";
import {inject, observer} from "mobx-react";
import FormControl from "react-bootstrap/FormControl";
import Form from "react-bootstrap/Form";
import ExplorerStore from "app/stores/ExplorerStore";

interface Props {
    nodeStore?: NodeStore;
    explorerStore?: ExplorerStore;
}

@inject("nodeStore")
@inject("explorerStore")
@observer
export class NavExplorerSearchbar extends React.Component<Props, any> {

    updateSearch = (e) => {
        this.props.explorerStore.updateSearch(e.target.value);
    };

    executeSearch = (e: KeyboardEvent) => {
        if (e.key !== 'Enter') return;
        this.props.explorerStore.searchAny();
    };

    render() {
        let {search, searching} = this.props.explorerStore;
        return (
            <Form inline as={"div"}>
                <FormControl
                    type="text" onChange={this.updateSearch}
                    placeholder="Search the Tangle..." value={search}
                    className=" mr-sm-2" disabled={searching}
                    onKeyUp={this.executeSearch}
                />
            </Form>
        );
    }
}
