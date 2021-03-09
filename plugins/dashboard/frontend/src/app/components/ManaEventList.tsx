import {inject, observer} from "mobx-react";
import * as React from "react";
import {Card, ListGroup} from "react-bootstrap";

interface Props {
    title: string;
    listItems;
}

@inject("manaStore")
@observer
export default class ManaEventList extends React.Component<Props, any> {
    render() {
        return (
            <Card>
                <Card.Body>
                    <Card.Title>
                        {this.props.title}
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