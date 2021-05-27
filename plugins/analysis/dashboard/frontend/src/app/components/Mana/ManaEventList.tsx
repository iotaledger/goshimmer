import {inject, observer} from "mobx-react";
import * as React from "react";
import {Card, ListGroup} from "react-bootstrap";

interface Props {
    title: string;
    listItems;
    since: Date;
}

@inject("manaStore")
@observer
export default class ManaEventList extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>
                        {this.props.title}{" "}
                        {this.props.since==null?  <i style={{fontSize: '0.8rem'}}>since genesis</i> : <i style={{fontSize: '0.8rem'}}>since {this.props.since.toLocaleString()}</i>}
                    </Card.Title>
                    <ListGroup style={{
                        fontSize: '0.8rem',
                        height: '300px',
                        overflowY: 'auto'
                    }}>
                        {this.props.listItems}
                    </ListGroup>
                </Card.Body>
            </Card>
        );
    }
}