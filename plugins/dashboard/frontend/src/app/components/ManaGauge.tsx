import {observer} from "mobx-react";
import * as React from "react";
import Card from "react-bootstrap/Card";
import UpArrow from "../../assets/up.svg";
import DownArrow from "../../assets/down.svg";


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
                    <Card.Title>{this.props.title}</Card.Title>
                    <h2>{displayManaUnit(currentValue)} {displayChangeIcon(currentValue, prevValue)}</h2>

                </Card.Body>
            </Card>
        );
    }
}

export function displayChangeIcon(cur: number, prev: number) {
    if (cur === prev) {return []}
    if (cur > prev)
    {
        return <img src={UpArrow} alt="Up Arrow" width={'40px'}/>
    } else {
        return <img src={DownArrow} alt="Down Arrow" width={'40px'} />
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
