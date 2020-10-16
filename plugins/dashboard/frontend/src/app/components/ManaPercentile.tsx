import {observer} from "mobx-react";
import * as React from "react";
import {ProgressBar} from "react-bootstrap";

interface Props {
    data;
}

@observer
export default class ManaPercentile extends React.Component<Props, any> {
    render() {
        return (
            <ProgressBar animated
                         now={this.props.data}
                         label={`${this.props.data.toFixed(2)}%`}
            />
        )
    }
}