import * as React from 'react';
import Container from "react-bootstrap/Container";
import {inject, observer} from "mobx-react";
import {Link} from 'react-router-dom';
import VisualizerStore from "app/stores/VisualizerStore";
import NodeStore from "app/stores/NodeStore";
import Badge from "react-bootstrap/Badge";

interface Props {
    visualizerStore?: VisualizerStore;
    nodeStore?: NodeStore;
}

@inject("visualizerStore")
@inject("nodeStore")
@observer
export class Visualizer extends React.Component<Props, any> {

    componentDidMount(): void {
        this.props.visualizerStore.start();
    }

    componentWillUnmount(): void {
        this.props.visualizerStore.stop();
    }

    render() {
        let {vertices, solidCount, selected} = this.props.visualizerStore;
        let {last_mps_metric} = this.props.nodeStore;
        return (
            <Container>
                <h3>Visualizer</h3>
                <p>
                    <Badge pill style={{background: "#5dbbc9", color: "white"}}>
                        Solid
                    </Badge>
                    {' '}
                    <Badge pill style={{background: "#565656", color: "white"}}>
                        Unsolid
                    </Badge>
                    {' '}
                    <Badge pill style={{background: "#db6073", color: "white"}}>
                        Tip
                    </Badge>
                    <br/>
                    Vertices: {vertices.size}, Solid: {solidCount},{' '}
                    Unsolid: {vertices.size - solidCount}, MPS: {last_mps_metric.mps}
                    <br/>
                    Selected: {selected ?
                    <Link to={`/explorer/message/${selected.id}`}>
                        {selected.id.substr(0, 10)}
                    </Link>
                    : "-"}, {' '}
                    Trunk/Branch:{' '}
                    {
                        selected && selected.trunk_id && selected.branch_id ?
                            <span>
                                <Link to={`/explorer/message/${selected.trunk_id}`}>
                                    {selected.trunk_id.substr(0, 10)}
                                </Link>
                                /
                                <Link to={`/explorer/message/${selected.branch_id}`}>
                                    {selected.branch_id.substr(0, 10)}
                                </Link>
                            </span>
                            : "-"}
                </p>

                <div className={"visualizer"} style={{
                    zIndex: -1, position: "absolute",
                    top: 0, left: 0,
                    width: "100%",
                    height: "100%",
                    background: "#ededed"
                }} id={"visualizer"}/>
            </Container>
        );
    }
}
