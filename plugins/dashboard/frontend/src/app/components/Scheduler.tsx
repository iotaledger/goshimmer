import * as React from 'react';
import NodeStore from "app/stores/NodeStore";
import Card from "react-bootstrap/Card";
import {inject, observer} from "mobx-react";
import BufferSizeChart from "app/components/BufferSizeChart";
import DeficitChart from "app/components/DeficitChart";

interface Props {
    nodeStore?: NodeStore;
}

@inject("nodeStore")
@observer
export default class Scheduler extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>Scheduler Metrics </Card.Title>
                    <small>
                        <hr/>
                        <div className={"row"}>
                            <div className={"col-3"}>
                                Rate:
                            </div>
                            <div className={"col-3"}>
                                {this.props.nodeStore.status.scheduler.rate}
                            </div>
                            <div className={"col-3"}>
                                Max buffer size:
                            </div>
                            <div className={"col-3"}>
                                {this.props.nodeStore.status.scheduler.maxBufferSize}
                            </div>
                        </div>
                    </small>
                    <hr/>
                    <BufferSizeChart/>
                    <hr/>
                    <DeficitChart/>

                </Card.Body>
            </Card>
        );
    }
}
