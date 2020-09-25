import {inject, observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import {Table} from "react-bootstrap";

interface Props {
    data;
    title;
}

@inject("manaStore")
@observer
export default class ManaRichest extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body style={{'overflow': 'auto', 'height':'300px'}}>
                    <Card.Title>{this.props.title}</Card.Title>
                    <Table>
                        <thead>
                        <tr>
                            <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>Rank</th>
                            <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>NodeID</th>
                            <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>Mana</th>
                        </tr>
                        </thead>
                        <tbody>
                        {this.props.data}
                        </tbody>
                    </Table>
                </Card.Body>
            </Card>
    );
    }
}