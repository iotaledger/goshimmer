import {observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import {Table} from "react-bootstrap";

interface Props {
    data;
    title;
}

@observer
export default class ManaLeaderboard extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body style={{'height':'300px', fontSize: '0.8rem',}}>
                    <Card.Title>{this.props.title}</Card.Title>
                    <Card.Body style={{'overflow':'auto', 'height':'90%', 'padding':'0px 0px 20px 0px'}}>
                        <Table>
                            <thead>
                            <tr>
                                <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>Rank</th>
                                <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>NodeID</th>
                                <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>Mana</th>
                                <th style={{'position': 'sticky', 'top': 0,'background': 'white'}}>% of All</th>
                            </tr>
                            </thead>
                            <tbody>
                            {this.props.data}
                            </tbody>
                        </Table>
                    </Card.Body>
                </Card.Body>
            </Card>
        );
    }
}