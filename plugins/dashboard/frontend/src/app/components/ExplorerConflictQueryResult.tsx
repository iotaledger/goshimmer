import * as React from 'react';
import Container from "react-bootstrap/Container";
import NodeStore from "app/stores/NodeStore";
import { inject, observer } from "mobx-react";
import ExplorerStore from "app/stores/ExplorerStore";
import ListGroup from "react-bootstrap/ListGroup";
import {resolveBase58ConflictID} from "app/utils/conflict";
import {resolveConfirmationState} from "app/utils/confirmation_state";


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
export class ExplorerConflictQueryResult extends React.Component<Props, any> {
    componentDidMount() {
        this.props.explorerStore.getConflict(this.props.match.params.id);
        this.props.explorerStore.getConflictChildren(this.props.match.params.id);
        this.props.explorerStore.getConflictConflicts(this.props.match.params.id);
        this.props.explorerStore.getConflictVoters(this.props.match.params.id);
    }

    componentWillUnmount() {
        this.props.explorerStore.reset();
    }
    render() {
        let {id} = this.props.match.params;
        let { query_err, conflict, conflictChildren, conflictConflicts, conflictVoters } = this.props.explorerStore;

        if (query_err) {
            return (
                <Container>
                    <h4>Conflict not found - 404</h4>
                    <span>{id}</span>
                </Container>
            );
        }
        return (
            <Container>
                <h4>Conflict</h4>
                {conflict && <ListGroup>
                    <ListGroup.Item>ID: {resolveBase58ConflictID(conflict.id)}</ListGroup.Item>
                    <ListGroup.Item>Parents:
                        <ListGroup>
                        {conflict.parents.map((p,i) => <ListGroup.Item key={i}><a href={`/explorer/conflict/${p}`}>{resolveBase58ConflictID(p)}</a></ListGroup.Item>)}
                        </ListGroup>
                    </ListGroup.Item>
                    {<ListGroup.Item>Conflicts:
                        {conflict.conflictIDs && <ListGroup>
                            {conflict.conflictIDs.map((c,i) => <ListGroup.Item key={i}><a href={`/explorer/output/${c}`}>{c}</a></ListGroup.Item>)}
                        </ListGroup>}
                    </ListGroup.Item>}
                    <ListGroup.Item>ConfirmationState: {resolveConfirmationState(conflict.confirmationState)}</ListGroup.Item>
                    <ListGroup.Item> Children:
                        {conflictChildren && <ListGroup>
                            {conflictChildren.childConflicts.map((c,i) => <ListGroup.Item key={i}><a href={`/explorer/conflict/${c.conflictID}`}>{resolveBase58ConflictID(c.conflictID)}</a></ListGroup.Item>)}
                        </ListGroup> }
                    </ListGroup.Item>
                    {<ListGroup.Item> Conflicts:
                            {conflictConflicts && <ListGroup>
                                {conflictConflicts.conflicts.map((c,i) => <div key={i}>
                                    OutputID: <a href={`/explorer/output/${c.outputID.base58}`}>{c.outputID.base58}</a>
                                    <ListGroup className={"mb-2"}>
                                        {c.conflictIDs.map((b,j) => <ListGroup.Item key={j}>
                                            <a href={`/explorer/conflict/${b}`}>{resolveBase58ConflictID(b)}</a>
                                        </ListGroup.Item>)}
                                    </ListGroup>
                                </div>)}
                            </ListGroup> }
                        </ListGroup.Item>}
                    <ListGroup.Item> Voters:
                        {conflictVoters && <ListGroup>
                            {conflictVoters.voters.map((s,i) => <ListGroup.Item key={s+i}>{s}</ListGroup.Item>)}
                        </ListGroup> }
                    </ListGroup.Item>
                </ListGroup>}
            </Container>
        )
    }
}
