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
                        Synced: {this.props.nodeStore.status.tangleTime.synced ? "Yes" : "No"} </Card.Title>
                    <small>
                        <div>
                            <hr/>
                            <div>Message: <Link
                                to={`/explorer/message/${this.props.nodeStore.status.tangleTime.messageID}`}>
                                {this.props.nodeStore.status.tangleTime.messageID}
                            </Link></div>
                            <div className={"row"}>
                                <div className={"col"}>
                                    Acceptance
                                    Time:&nbsp;&nbsp;&nbsp;{dateformat(new Date(this.props.nodeStore.status.tangleTime.AT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                                <div className={"col"}>
                                    Relative Acceptance
                                    Time:&nbsp;&nbsp;&nbsp;{dateformat(new Date(this.props.nodeStore.status.tangleTime.RAT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                            </div>
                            <div className={"row"}>
                                <div className={"col"}>
                                    Confirmation
                                    Time: {dateformat(new Date(this.props.nodeStore.status.tangleTime.CT / 1000000), "dd.mm.yyyy HH:MM:ss")}
                                </div>
                                <div className={"col"}>
                                    Relative Confirmation
                                    Time: {dateformat(new Date(this.props.nodeStore.status.tangleTime.RCT / 1000000), "dd.mm.yyyy HH:MM:ss")}
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
