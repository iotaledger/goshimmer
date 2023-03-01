import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import Card from "react-bootstrap/Card";
import {Link} from 'react-router-dom';
import {inject, observer} from "mobx-react";
import * as dateformat from 'dateformat';

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class TangleTime extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>TangleTime
                        Synced: {this.props.nodeStore.status.tangleTime.synced ? "Yes" : "No"}</Card.Title>
                    <small>
                        <div>
                            <hr/>
                            <div className={"row"}>
                                <div className={"col-12"}>Last Accepted Block: <Link
                                    to={`/explorer/block/${this.props.nodeStore.status.tangleTime.acceptedBlockID}`}>
                                    {this.props.nodeStore.status.tangleTime.acceptedBlockID}
                                </Link></div>
                            </div>
                            <div className={"row"}>
                                <div className={"col-12"}>Last Confirmed Block: <Link
                                    to={`/explorer/block/${this.props.nodeStore.status.tangleTime.confirmedBlockID}`}>
                                    {this.props.nodeStore.status.tangleTime.confirmedBlockID}
                                </Link></div>
                            </div>
                            <div className={"row"}>
                                <div className={"col-12"}>Last Confirmed Slot:
                                    {this.props.nodeStore.status.tangleTime.confirmedSlot}
                                </div>
                            </div>
                            <hr/>
                            <div className={"row"}>
                                <div className={"col-3"}>
                                    Acceptance Time:
                                </div>
                                <div className={"col-3"}>
                                    {dateformat(new Date(this.props.nodeStore.status.tangleTime.ATT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                                <div className={"col-3"}>
                                    Confirmation Time:
                                </div>
                                <div className={"col-3"}>
                                    {dateformat(new Date(this.props.nodeStore.status.tangleTime.CTT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                            </div>
                            <div className={"row"}>
                                <div className={"col-3"}>
                                    Relative Acceptance Time:
                                </div>
                                <div className={"col-3"}>
                                    {dateformat(new Date(this.props.nodeStore.status.tangleTime.RATT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                                <div className={"col-3"}>
                                    Relative Confirmation Time:
                                </div>
                                <div className={"col-3"}>
                                    {dateformat(new Date(this.props.nodeStore.status.tangleTime.RCTT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                            </div>
                        </div>
                    </small>
                </Card.Body>
            </Card>
        )
            ;
    }
}
