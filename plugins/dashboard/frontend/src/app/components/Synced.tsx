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
export default class Synced extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Synced: {this.props.nodeStore.status.synced? "Yes":"No"}</Card.Title>
                    <small>           
                        {Object.keys(this.props.nodeStore.status.beacons).map(nodeID =>
                            <div>
                                <hr/>
                                <div><strong>Public Key: {nodeID}</strong></div> 
                                <div>Synced: {this.props.nodeStore.status.beacons[nodeID].synced? "Yes":"No"}</div> 
                                <div>Beacon: <Link to={`/explorer/message/${this.props.nodeStore.status.beacons[nodeID].msg_id}`}>
                                                {this.props.nodeStore.status.beacons[nodeID].msg_id}
                                            </Link></div>
                                <div>Time: {dateformat(new Date(this.props.nodeStore.status.beacons[nodeID].sent_time/1000000), "dd.mm.yyyy HH:MM:ss")}</div>
                            </div>
                        )}
                    </small>

                </Card.Body>
            </Card>
        );
    }
}
