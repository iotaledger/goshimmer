import {observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import UpArrow from "../../assets/up.svg";
import DownArrow from "../../assets/down.svg";
import {Col, Container, Row} from "react-bootstrap";


interface Props {
    title: string;
    data: Array<number>;
}

@observer
export default class ManaGauge extends React.Component<Props, any> {
    render() {
        const currentValue = this.props.data[0];
        const prevValue = this.props.data[1];
        return (
            <Card>
                <Card.Body>
                    <Card.Title style={{marginBottom: '0rem'}}>
                        <Container fluid style={{padding: '0rem'}}>
                            <Row>
                                <Col>
                                    {this.props.title}
                                </Col>
                                <Col>
                                    <b>{displayManaUnit(currentValue)} {displayChangeIcon(currentValue, prevValue)}</b>
                                </Col>
                            </Row>
                        </Container>
                    </Card.Title>
                </Card.Body>
            </Card>
        );
    }
}

export function displayChangeIcon(cur: number, prev: number) {
    if (cur === prev) {return []}
    if (cur > prev)
    {
        return <img src={UpArrow} alt="Up Arrow" width={'20px'}/>
    } else {
        return <img src={DownArrow} alt="Down Arrow" width={'20px'} />
    }
}

export function displayManaUnit(mana: number): string {
    let result = ""
    // round to nearest integer
    let roundedMana = Math.round(mana);
    if (roundedMana < 1000) {
        result = roundedMana.toString(10) + " m"; // mana
    } else if (roundedMana < 1000000) {
        result = (roundedMana / 1000).toFixed(3) + " Km"; // kilomana
    }
    else if (roundedMana < 1000000000) {
        result = (roundedMana / 1000000).toFixed(3) + " Mm"; // megamana
    }
    else if (roundedMana < 1000000000000) {
        result = (roundedMana / 1000000000).toFixed(3) + " Gm"; // gigamana
    }
    else if (roundedMana < 1000000000000000) {
        result = (roundedMana / 1000000000000).toFixed(3) + " Tm"; // terramana
    } else {
        result = (roundedMana / 1000000000000000).toFixed(3) + " Pm"; // petamana
    }
    return result
}
