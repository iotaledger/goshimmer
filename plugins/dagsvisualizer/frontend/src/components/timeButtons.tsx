import * as React from 'react';
import Button from 'react-bootstrap/Button';
import GlobalStore from '../stores/GlobalStore';
import {inject, observer} from 'mobx-react';
import moment, {isMoment} from 'moment';
import Datetime from 'react-datetime';
import {DATE_FORMAT, TIME_FORMAT} from '../utils/constants';

interface Props {
    globalStore?: GlobalStore;
    isStartTime: boolean;
}

@inject('globalStore')
@observer
export class TimePickerButtons extends React.Component<Props, any> {

    render() {

        const isStartTime = this.props.isStartTime;
        const picker = <>
            <Picker isStartTime={this.props.isStartTime} globalStore={this.props.globalStore}/>
        </>;
        const setCurrent = () => {
            const m = moment();
            if (isStartTime) {
                this.props.globalStore.updateSearchStartingTime(m);

            } else {
                this.props.globalStore.updateSearchEndingTime(m);


            }
        };

        return (
            <div>
                {picker}
                <div
                    className={'tripleButton'}>
                    <Button
                        className={'button button-wide button-left'}
                        variant="outline-secondary"
                        onClick={setCurrent}
                    >
                        Set current time
                    </Button>
                    <Button
                        className={'button button-middle'}
                        variant="outline-secondary"
                    >
                        -30s
                    </Button>
                    <Button
                        className={'button button-right'}
                        variant="outline-secondary"
                    >
                        +30s
                    </Button>
                </div>
            </div>
        );
    }
}

interface PickerProps {
    globalStore?: GlobalStore;
    isStartTime: boolean;
    action?: string;
}

export class Picker extends React.Component<PickerProps, any> {

    updateFrom = date => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchStartingTime(date);
        }
    };

    updateTo = date => {
        if (isMoment(date)) {
            this.props.globalStore.updateSearchEndingTime(date);
        }
    };

    render() {
        const isStartTime = this.props.isStartTime;
        const updateFunc = isStartTime ? this.updateTo : this.updateFrom;

        return (
            <Datetime
                dateFormat={DATE_FORMAT}
                timeFormat={TIME_FORMAT}
                onChange={updateFunc}
                // value={}
                // this.props.action === '' ? /
            />
        );
    }
}